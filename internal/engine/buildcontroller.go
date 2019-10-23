package engine

import (
	"context"
	"sort"
	"time"

	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type BuildController struct {
	b                  BuildAndDeployer
	lastActionCount    int
	disabledForTesting bool
}

type buildEntry struct {
	name          model.ManifestName
	targets       []model.TargetSpec
	buildStateSet store.BuildStateSet
	filesChanged  []string
	buildReason   model.BuildReason
	firstBuild    bool
}

func NewBuildController(b BuildAndDeployer) *BuildController {
	return &BuildController{
		b:               b,
		lastActionCount: -1,
	}
}

func (c *BuildController) needsBuild(ctx context.Context, st store.RStore) (buildEntry, bool) {
	state := st.RLockState()
	defer st.RUnlockState()

	// Don't start the next build until the previous action has been recorded,
	// so that we don't accidentally repeat the same build.
	if c.lastActionCount == state.BuildControllerActionCount {
		return buildEntry{}, false
	}

	mt := buildcontrol.NextTargetToBuild(state)
	if mt == nil {
		return buildEntry{}, false
	}

	c.lastActionCount = state.BuildControllerActionCount
	ms := mt.State
	manifest := mt.Manifest
	firstBuild := !ms.StartedFirstBuild()

	buildReason := ms.NextBuildReason()
	targets := buildTargets(manifest)
	buildStateSet := buildStateSet(ctx, manifest, targets, ms)

	return buildEntry{
		name:          manifest.Name,
		targets:       targets,
		firstBuild:    firstBuild,
		buildReason:   buildReason,
		buildStateSet: buildStateSet,
		filesChanged:  append(ms.ConfigFilesThatCausedChange, buildStateSet.FilesChanged()...),
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

	go func() {
		// Send the logs to both the EngineState and the normal log stream.
		actionWriter := BuildLogActionWriter{
			store:        st,
			manifestName: entry.name,
		}
		ctx := logger.WithLogger(ctx, logger.NewLogger(logger.Get(ctx).Level(), actionWriter))

		st.Dispatch(BuildStartedAction{
			ManifestName: entry.name,
			StartTime:    time.Now(),
			FilesChanged: entry.filesChanged,
			Reason:       entry.buildReason,
		})
		c.logBuildEntry(ctx, entry)

		result, err := c.buildAndDeploy(ctx, st, entry)
		st.Dispatch(NewBuildCompleteAction(result, err))
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

func (c *BuildController) logBuildEntry(ctx context.Context, entry buildEntry) {
	firstBuild := entry.firstBuild
	name := entry.name
	changedFiles := entry.filesChanged

	l := logger.Get(ctx)
	if firstBuild {
		p := logger.Blue(l).Sprintf("──┤ Building: ")
		s := logger.Blue(l).Sprintf(" ├──────────────────────────────────────────────")
		l.Infof("\n%s%s%s", p, name, s)
	} else {
		if len(changedFiles) > 0 {
			p := logger.Green(l).Sprintf("%d changed: ", len(changedFiles))
			l.Infof("\n%s%v\n", p, ospath.FormatFileChangeList(changedFiles))
		}

		rp := logger.Blue(l).Sprintf("──┤ Rebuilding: ")
		rs := logger.Blue(l).Sprintf(" ├────────────────────────────────────────────")
		l.Infof("\n%s%s%s", rp, name, rs)
	}
}

type BuildLogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName
}

func (w BuildLogActionWriter) Write(p []byte) (n int, err error) {
	w.store.Dispatch(BuildLogAction{
		LogEvent: store.NewLogEvent(w.manifestName, p),
	})
	return len(p), nil
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
	buildStateSet := store.BuildStateSet{}

	for _, spec := range specs {
		id := spec.ID()
		if id.Type != model.TargetTypeImage && id.Type != model.TargetTypeDockerCompose && id.Type != model.TargetTypeLocal {
			continue
		}

		status := ms.BuildStatus(id)
		filesChanged := make([]string, 0, len(status.PendingFileChanges))
		for file, _ := range status.PendingFileChanges {
			filesChanged = append(filesChanged, file)
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
		buildStateSet[id] = buildState
	}

	return buildStateSet
}

var _ store.Subscriber = &BuildController{}
