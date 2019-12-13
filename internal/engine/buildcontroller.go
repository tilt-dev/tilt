package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type BuildController struct {
	b                  BuildAndDeployer
	buildsStartedCount int // used to synchronize with state
	disabledForTesting bool
}

type buildEntry struct {
	name          model.ManifestName
	targets       []model.TargetSpec
	buildStateSet store.BuildStateSet
	filesChanged  []string
	buildReason   model.BuildReason
	firstBuild    bool
	buildCount    int
}

func NewBuildController(b BuildAndDeployer) *BuildController {
	return &BuildController{
		b:                  b,
		buildsStartedCount: 0,
	}
}

func (c *BuildController) needsBuild(ctx context.Context, st store.RStore) (buildEntry, bool) {
	state := st.RLockState()
	defer st.RUnlockState()

	// Don't start the next build until the previous action has been recorded,
	// so that we don't accidentally repeat the same build.
	if c.buildsStartedCount != state.StartedBuildCount {
		return buildEntry{}, false
	}

	// no build slots available
	if state.AvailableBuildSlots() < 1 {
		return buildEntry{}, false
	}

	mt := buildcontrol.NextTargetToBuild(state)
	if mt == nil {
		return buildEntry{}, false
	}

	c.buildsStartedCount += 1
	ms := mt.State
	manifest := mt.Manifest
	firstBuild := !ms.StartedFirstBuild()

	buildReason := mt.NextBuildReason()
	targets := buildTargets(manifest)
	buildStateSet := buildStateSet(ctx, manifest, targets, ms)

	return buildEntry{
		name:          manifest.Name,
		targets:       targets,
		firstBuild:    firstBuild,
		buildReason:   buildReason,
		buildStateSet: buildStateSet,
		filesChanged:  append(ms.ConfigFilesThatCausedChange, buildStateSet.FilesChanged()...),
		buildCount:    c.buildsStartedCount,
	}, true
}

func (c *BuildController) DisableForTesting() {
	c.disabledForTesting = true
}

func (c *BuildController) OnChange(ctx context.Context, st store.RStore) {
	if c.disabledForTesting {
		return
	}
	entry, ok := c.needsBuild(ctx, st)
	if !ok {
		return
	}

	st.Dispatch(buildcontrol.BuildStartedAction{
		ManifestName: entry.name,
		StartTime:    time.Now(),
		FilesChanged: entry.filesChanged,
		Reason:       entry.buildReason,
		SpanID:       SpanIDForBuildLog(entry.buildCount),
	})

	go func() {
		// Send the logs to both the EngineState and the normal log stream.
		actionWriter := BuildLogActionWriter{
			store:        st,
			manifestName: entry.name,
			buildCount:   entry.buildCount,
		}
		ctx := logger.CtxWithLogHandler(ctx, actionWriter)

		c.logBuildEntry(ctx, entry)

		result, err := c.buildAndDeploy(ctx, st, entry)
		st.Dispatch(buildcontrol.NewBuildCompleteAction(entry.name, result, err))
	}()
}

func (c *BuildController) buildAndDeploy(ctx context.Context, st store.RStore, entry buildEntry) (store.BuildResultSet, error) {
	targets := entry.targets
	for _, target := range targets {
		err := target.Validate()
		if err != nil {
			return store.BuildResultSet{}, err
		}
	}
	return c.b.BuildAndDeploy(ctx, st, targets, entry.buildStateSet)
}

// ideally narrow enough that it doesn't wrap on "normal" resolutions
const buildLineLength = 55

// if name is long, then the whole thing won't even fit in buildLineLength. Show at least minSuffixLineLength
// ──┤ Rebuilding: ├────────
const minSuffixLineLength = 8

func (c *BuildController) logBuildEntry(ctx context.Context, entry buildEntry) {
	firstBuild := entry.firstBuild
	name := entry.name
	changedFiles := entry.filesChanged

	l := logger.Get(ctx)
	var prefix string
	if firstBuild {
		prefix = "──┤ Building:"
	} else {
		if len(changedFiles) > 0 {
			p := logger.Green(l).Sprintf("%d changed: ", len(changedFiles))
			l.Infof("\n%s%v\n", p, ospath.FormatFileChangeList(changedFiles))
		}

		prefix = "──┤ Rebuilding: "
	}

	p := logger.Blue(l).Sprintf("%s", prefix)
	trailingDashCount := buildLineLength - len(name) - len(prefix) - 2
	if trailingDashCount < minSuffixLineLength {
		trailingDashCount = minSuffixLineLength
	}
	suffix := " ├" + strings.Repeat("─", trailingDashCount)
	s := logger.Blue(l).Sprintf(suffix)
	l.Infof("\n%s%s%s", p, name, s)
}

type BuildLogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName
	buildCount   int
}

func (w BuildLogActionWriter) Write(level logger.Level, p []byte) error {
	w.store.Dispatch(buildcontrol.BuildLogAction{
		LogEvent: store.NewLogEvent(w.manifestName, SpanIDForBuildLog(w.buildCount), level, p),
	})
	return nil
}

func SpanIDForBuildLog(buildCount int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("build:%d", buildCount))
}

// Extract target specs from a manifest for BuildAndDeploy.
func buildTargets(manifest model.Manifest) []model.TargetSpec {
	var result []model.TargetSpec

	for _, iTarget := range manifest.ImageTargets {
		result = append(result, iTarget)
	}

	if manifest.IsDC() {
		result = append(result, manifest.DockerComposeTarget())
	} else if manifest.IsK8s() {
		result = append(result, manifest.K8sTarget())
	} else if manifest.IsLocal() {
		result = append(result, manifest.LocalTarget())
	}

	return result
}

// Extract a set of build states from a manifest for BuildAndDeploy.
func buildStateSet(ctx context.Context, manifest model.Manifest, specs []model.TargetSpec, ms *store.ManifestState) store.BuildStateSet {
	result := store.BuildStateSet{}

	anyFilesChangedSinceLastBuild := false

	for _, spec := range specs {
		id := spec.ID()
		if id.Type != model.TargetTypeImage && id.Type != model.TargetTypeDockerCompose && id.Type != model.TargetTypeLocal {
			continue
		}

		status := ms.BuildStatus(id)
		var filesChanged []string
		for file, ts := range status.PendingFileChanges {
			filesChanged = append(filesChanged, file)
			if ms.LastBuild().Empty() || ts.After(ms.LastBuild().StartTime) {
				anyFilesChangedSinceLastBuild = true
			}
		}
		sort.Strings(filesChanged)

		buildState := store.NewBuildState(status.LastSuccessfulResult, filesChanged)

		// Pass along the container when we can update containers in-place.
		//
		// We don't want to pass along the data if the pod is crashing, because
		// we're not confident that this state is accurate, due to how orchestrators
		// (like k8s) reschedule containers (i.e., they reset to the original image
		// rather than persisting the container filesystem.)
		//
		// This will probably need to change as the mapping between containers and
		// manifests becomes many-to-one.
		if !ms.NeedsRebuildFromCrash {
			iTarget, ok := spec.(model.ImageTarget)
			if ok {
				if manifest.IsK8s() {
					cInfos, err := store.RunningContainersForTargetForOnePod(iTarget, ms.K8sRuntimeState())
					if err != nil {
						buildState = buildState.WithRunningContainerError(err)
					} else {
						buildState = buildState.WithRunningContainers(cInfos)
					}
				}

				if manifest.IsDC() {
					buildState = buildState.WithRunningContainers(store.RunningContainersForDC(ms.DCRuntimeState()))
				}
			}
		}
		result[id] = buildState
	}

	// If there are no files changed across the entire state set, then this is a force update.
	// We want to do an image build of each image.
	// TODO(maia): I think that instead of storing this on every build state, we can can figure
	//  out that it's a force update when creating the BuildEntry (in `needsBuild`), store that
	//  as the BuildReason, and pass the whole BuildEntry to the builder (so the builder can
	//  know whether to skip in-place builds)
	// if we're on a crash rebuild, then there won't have been any files changed for that reason
	if !ms.NeedsRebuildFromCrash && !anyFilesChangedSinceLastBuild {
		for k, v := range result {
			result[k] = v.WithNeedsForceUpdate(true)
		}
	}

	return result
}

var _ store.Subscriber = &BuildController{}
