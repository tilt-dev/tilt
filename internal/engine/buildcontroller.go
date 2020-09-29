package engine

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
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
	spanID        logstore.SpanID
}

func (e buildEntry) Name() model.ManifestName       { return e.name }
func (e buildEntry) FilesChanged() []string         { return e.filesChanged }
func (e buildEntry) BuildReason() model.BuildReason { return e.buildReason }

func NewBuildController(b BuildAndDeployer) *BuildController {
	return &BuildController{
		b: b,
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

	mt, _ := buildcontrol.NextTargetToBuild(state)
	if mt == nil {
		return buildEntry{}, false
	}

	c.buildsStartedCount += 1
	ms := mt.State
	manifest := mt.Manifest

	buildReason := mt.NextBuildReason()
	targets := buildTargets(manifest)
	buildStateSet := buildStateSet(ctx, manifest, targets, ms, buildReason)

	return buildEntry{
		name:          manifest.Name,
		targets:       targets,
		buildReason:   buildReason,
		buildStateSet: buildStateSet,
		filesChanged:  append(ms.ConfigFilesThatCausedChange, buildStateSet.FilesChanged()...),
		spanID:        SpanIDForBuildLog(c.buildsStartedCount),
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
		SpanID:       entry.spanID,
	})

	go func() {
		// Send the logs to both the EngineState and the normal log stream.
		actionWriter := BuildLogActionWriter{
			store:        st,
			manifestName: entry.name,
			spanID:       entry.spanID,
		}
		ctx := logger.CtxWithLogHandler(ctx, actionWriter)

		buildcontrol.LogBuildEntry(ctx, entry)

		result, err := c.buildAndDeploy(ctx, st, entry)
		st.Dispatch(buildcontrol.NewBuildCompleteAction(entry.name, entry.spanID, result, err))
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

type BuildLogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName
	spanID       logstore.SpanID
}

func (w BuildLogActionWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.store.Dispatch(store.NewLogAction(w.manifestName, w.spanID, level, fields, p))
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
func buildStateSet(ctx context.Context, manifest model.Manifest, specs []model.TargetSpec,
	ms *store.ManifestState, reason model.BuildReason) store.BuildStateSet {
	result := store.BuildStateSet{}

	for _, spec := range specs {
		id := spec.ID()
		status := ms.BuildStatus(id)
		var filesChanged []string
		for file := range status.PendingFileChanges {
			filesChanged = append(filesChanged, file)
		}
		sort.Strings(filesChanged)

		var depsChanged []model.TargetID
		for dep := range status.PendingDependencyChanges {
			depsChanged = append(depsChanged, dep)
		}

		buildState := store.NewBuildState(status.LastResult, filesChanged, depsChanged)

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

	isLiveUpdateEligibleTrigger := reason.HasTrigger() &&
		reason.Has(model.BuildReasonFlagChangedFiles) &&
		manifest.TriggerMode != model.TriggerModeAuto
	isFullBuildTrigger := reason.HasTrigger() && !isLiveUpdateEligibleTrigger
	if isFullBuildTrigger {
		for k, v := range result {
			result[k] = v.WithFullBuildTriggered(true)
		}
	}

	return result
}

var _ store.Subscriber = &BuildController{}
