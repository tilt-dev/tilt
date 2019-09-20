package engine

import (
	"context"
	"sort"
	"time"

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

// Algorithm to choose a manifest to build next.
func nextTargetToBuild(state store.EngineState) *store.ManifestTarget {
	// Don't build anything if there are pending config file changes.
	// We want the Tiltfile to re-run first.
	if len(state.PendingConfigFileChanges) > 0 {
		return nil
	}

	targets := state.Targets()

	// If any of the manifest target's haven't been built yet, build them now.
	unbuilt := findUnbuiltTargets(targets)
	if len(unbuilt) > 0 {
		return nextUnbuiltTargetToBuild(unbuilt)
	}

	// Next prioritize builds that crashed and need a rebuilt to have up-to-date code.
	for _, mt := range targets {
		if mt.State.NeedsRebuildFromCrash {
			return mt
		}
	}

	// Next prioritize builds that are have been manually triggered.
	if len(state.TriggerQueue) > 0 {
		mn := state.TriggerQueue[0]
		mt, ok := state.ManifestTargets[mn]
		if ok {
			return mt
		}
	}

	return earliestPendingAutoTriggerTarget(targets)
}

// Go through all the manifests, and check:
// 1) all pending file changes, and
// 2) all pending manifest changes
// The earliest one is the one we want.
//
// If no targets are pending, return nil
func earliestPendingAutoTriggerTarget(targets []*store.ManifestTarget) *store.ManifestTarget {
	var choice *store.ManifestTarget
	earliest := time.Now()

	for _, mt := range targets {
		ok, newTime := mt.State.HasPendingChangesBefore(earliest)
		if ok {
			if mt.Manifest.TriggerMode == model.TriggerModeManual {
				// Don't trigger update of a manual manifest just b/c if has
				// pending changes; must come through the TriggerQueue, above.
				continue
			}
			choice = mt
			earliest = newTime
		}
	}

	return choice
}

// Helper function for ordering targets that have never been built before.
func nextUnbuiltTargetToBuild(unbuilt []*store.ManifestTarget) *store.ManifestTarget {
	// unresourced YAML goes first
	unresourced := findUnresourcedYAML(unbuilt)
	if unresourced != nil {
		return unresourced
	}

	// If this is Kubernetes, unbuilt resources go first.
	// (If this is Docker Compose, we want to trust the ordering
	// that docker-compose put things in.)
	deployOnlyK8sTargets := findDeployOnlyK8sManifestTargets(unbuilt)
	if len(deployOnlyK8sTargets) > 0 {
		return deployOnlyK8sTargets[0]
	}

	return unbuilt[0]
}

func findUnbuiltTargets(targets []*store.ManifestTarget) []*store.ManifestTarget {
	result := []*store.ManifestTarget{}
	for _, target := range targets {
		if !target.State.StartedFirstBuild() {
			result = append(result, target)
		}
	}
	return result
}

func findUnresourcedYAML(targets []*store.ManifestTarget) *store.ManifestTarget {
	for _, target := range targets {
		if target.Manifest.ManifestName() == model.UnresourcedYAMLManifestName {
			return target
		}
	}
	return nil
}

func findDeployOnlyK8sManifestTargets(targets []*store.ManifestTarget) []*store.ManifestTarget {
	result := []*store.ManifestTarget{}
	for _, target := range targets {
		if target.Manifest.IsK8s() && len(target.Manifest.ImageTargets) == 0 {
			result = append(result, target)
		}
	}
	return result
}

func nextManifestNameToBuild(state store.EngineState) model.ManifestName {
	mt := nextTargetToBuild(state)
	if mt == nil {
		return ""
	}
	return mt.Manifest.Name
}

func (c *BuildController) needsBuild(ctx context.Context, st store.RStore) (buildEntry, bool) {
	state := st.RLockState()
	defer st.RUnlockState()

	// Don't start the next build until the previous action has been recorded,
	// so that we don't accidentally repeat the same build.
	if c.lastActionCount == state.BuildControllerActionCount {
		return buildEntry{}, false
	}

	mt := nextTargetToBuild(state)
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
			l.Infof("\n%s%v\n", p, formatFileChangeList(changedFiles))
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
		if id.Type != model.TargetTypeImage && id.Type != model.TargetTypeDockerCompose {
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
						// Couldn't get running container info; surface an error IF target has LiveUpdate instructions.
						if !iTarget.AnyFastBuildInfo().Empty() || !iTarget.AnyLiveUpdateInfo().Empty() {
							logger.Get(ctx).Infof("CANNOT PERFORM LIVE UPDATE ON IMAGE %s:\n\t%v", iTarget.ID(), err)
						}
					}
					buildState = buildState.WithRunningContainers(cInfos)
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
