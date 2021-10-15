package engine

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/dcconv"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type BuildController struct {
	b                  buildcontrol.BuildAndDeployer
	buildsStartedCount int // used to synchronize with state
	disabledForTesting bool

	// CancelFuncs for in-progress builds
	mu           sync.Mutex
	cancelBuilds map[model.ManifestName]context.CancelFunc
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

func NewBuildController(b buildcontrol.BuildAndDeployer) *BuildController {
	return &BuildController{
		b:            b,
		cancelBuilds: make(map[model.ManifestName]context.CancelFunc),
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
	targets := buildcontrol.BuildTargets(manifest)
	buildStateSet := buildStateSet(ctx, manifest, state.KubernetesResources[manifest.Name.String()],
		targets, ms, buildReason)

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

func (c *BuildController) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	c.cleanupDisabledBuilds(st)

	if c.disabledForTesting {
		return nil
	}
	entry, ok := c.needsBuild(ctx, st)
	if !ok {
		return nil
	}

	st.Dispatch(buildcontrol.BuildStartedAction{
		ManifestName:       entry.name,
		StartTime:          time.Now(),
		FilesChanged:       entry.filesChanged,
		Reason:             entry.buildReason,
		SpanID:             entry.spanID,
		FullBuildTriggered: entry.buildStateSet.FullBuildTriggered(),
	})

	go func() {
		ctx = c.buildContext(ctx, entry, st)
		defer c.cleanupBuildContext(entry.name)

		buildcontrol.LogBuildEntry(ctx, buildcontrol.BuildEntry{
			Name:         entry.Name(),
			BuildReason:  entry.BuildReason(),
			FilesChanged: entry.FilesChanged(),
		})

		result, err := c.buildAndDeploy(ctx, st, entry)
		st.Dispatch(buildcontrol.NewBuildCompleteAction(entry.name, entry.spanID, result, err))
	}()

	return nil
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

// cancel any in-progress builds associated with disabled UIResources
// when builds are fully represented by api objects, cancellation should probably
// be tied to those rather than the UIResource
func (c *BuildController) cleanupDisabledBuilds(st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()

	for _, uir := range state.UIResources {
		if uir.Status.DisableStatus.DisabledCount > 0 {
			c.cleanupBuildContext(model.ManifestName(uir.Name))
		}
	}
}

func (c *BuildController) buildContext(ctx context.Context, entry buildEntry, st store.RStore) context.Context {
	// Send the logs to both the EngineState and the normal log stream.
	actionWriter := BuildLogActionWriter{
		store:        st,
		manifestName: entry.name,
		spanID:       entry.spanID,
	}
	ctx = logger.CtxWithLogHandler(ctx, actionWriter)

	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancelBuilds[entry.name] = cancel
	return ctx
}

func (c *BuildController) cleanupBuildContext(mn model.ManifestName) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cancel, ok := c.cancelBuilds[mn]; ok {
		cancel()
		delete(c.cancelBuilds, mn)
	}
}

func SpanIDForBuildLog(buildCount int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("build:%d", buildCount))
}

// Extract a set of build states from a manifest for BuildAndDeploy.
func buildStateSet(ctx context.Context, manifest model.Manifest,
	kresource *k8sconv.KubernetesResource, specs []model.TargetSpec,
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
				selector := iTarget.LiveUpdateSpec.Selector
				if manifest.IsK8s() && selector.Kubernetes != nil {
					buildState.KubernetesSelector = selector.Kubernetes
					buildState.KubernetesResource = kresource
				}

				if manifest.IsDC() {
					buildState.DockerResource = &dcconv.DockerResource{ContainerID: string(ms.DCRuntimeState().ContainerID)}
				}
			}
		}
		result[id] = buildState
	}

	// If the user manually triggers the build and there are no pending changes,
	// assume they want a full build, since there are there no file changes to sync.
	// (If there ARE pending changes but the resource is automatic, then a LiveUpdate
	// (if configured) is already queued, so assume the user wants to trigger a
	// full build instead.)
	isLiveUpdateEligibleTrigger := reason.HasTrigger() &&
		reason.Has(model.BuildReasonFlagChangedFiles) &&
		!manifest.TriggerMode.AutoOnChange()
	isFullBuildTrigger := reason.HasTrigger() && !isLiveUpdateEligibleTrigger
	if isFullBuildTrigger {
		for k, v := range result {
			result[k] = v.WithFullBuildTriggered(true)
		}
	}

	return result
}

var _ store.Subscriber = &BuildController{}
