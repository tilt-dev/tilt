package engine

import (
	"context"
	"sort"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
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
	// First, go through all the manifests in order.
	// If any of them haven't started yet, build them now.
	for _, mt := range state.Targets() {
		if !mt.State.StartedFirstBuild() {
			return mt
		}
	}

	// Next go through all the manifests, and check:
	// 1) all pending file changes, and
	// 2) all pending manifest changes
	// The earliest one is the one we want.
	var choice *store.ManifestTarget
	earliest := time.Now()

	// always use a stable iteration order
	for _, mt := range state.Targets() {
		// Always prioritize builds that crashes and have an out-of-sync.
		if mt.State.NeedsRebuildFromCrash {
			return mt
		}
	}

	if state.TriggerMode == model.TriggerManual && len(state.TriggerQueue) > 0 {
		mn := state.TriggerQueue[0]
		mt, ok := state.ManifestTargets[mn]
		if ok {
			return mt
		}
	}

	if state.TriggerMode == model.TriggerAuto {
		for _, mt := range state.Targets() {
			ok, newTime := mt.State.HasPendingChangesBefore(earliest)
			if ok {
				choice = mt
				earliest = newTime
			}
		}
	}

	return choice
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
	buildStateSet := buildStateSet(manifest, targets, ms)

	return buildEntry{
		name:          manifest.Name,
		targets:       targets,
		firstBuild:    firstBuild,
		buildReason:   buildReason,
		buildStateSet: buildStateSet,
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
		ctx = logger.CtxWithForkedOutput(ctx, actionWriter)

		filesChanged := entry.buildStateSet.FilesChanged()
		st.Dispatch(BuildStartedAction{
			ManifestName: entry.name,
			StartTime:    time.Now(),
			FilesChanged: filesChanged,
			Reason:       entry.buildReason,
		})
		c.logBuildEntry(ctx, entry, filesChanged)

		result, err := c.buildAndDeploy(ctx, entry)
		st.Dispatch(NewBuildCompleteAction(result, err))
	}()
}

func (c *BuildController) buildAndDeploy(ctx context.Context, entry buildEntry) (store.BuildResultSet, error) {
	targets := entry.targets
	for _, target := range targets {
		err := target.Validate()
		if err != nil {
			return store.BuildResultSet{}, err
		}
	}
	return c.b.BuildAndDeploy(ctx, targets, entry.buildStateSet)
}

func (c *BuildController) logBuildEntry(ctx context.Context, entry buildEntry, changedFiles []string) {
	firstBuild := entry.firstBuild
	name := entry.name

	l := logger.Get(ctx)
	if firstBuild {
		p := logger.Blue(l).Sprintf("\n──┤ Building: ")
		s := logger.Blue(l).Sprintf(" ├──────────────────────────────────────────────")
		l.Infof("%s%s%s", p, name, s)
	} else {
		var changedPathsToPrint []string
		if len(changedFiles) > maxChangedFilesToPrint {
			changedPathsToPrint = append(changedPathsToPrint, changedFiles[:maxChangedFilesToPrint]...)
			changedPathsToPrint = append(changedPathsToPrint, "...")
		} else {
			changedPathsToPrint = changedFiles
		}

		if len(changedFiles) > 0 {
			p := logger.Green(l).Sprintf("\n%d changed: ", len(changedFiles))
			l.Infof("%s%v\n", p, ospath.TryAsCwdChildren(changedPathsToPrint))
		}

		rp := logger.Blue(l).Sprintf("\n──┤ Rebuilding: ")
		rs := logger.Blue(l).Sprintf(" ├────────────────────────────────────────────")
		l.Infof("%s%s%s", rp, name, rs)
	}
}

type BuildLogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName
}

func (w BuildLogActionWriter) Write(p []byte) (n int, err error) {
	w.store.Dispatch(BuildLogAction{
		ManifestName: w.manifestName,
		Log:          append([]byte{}, p...),
	})
	return len(p), nil
}

// Extract target specs from a manifest for BuildAndDeploy.
func buildTargets(manifest model.Manifest) []model.TargetSpec {
	if manifest.IsDC() {
		return []model.TargetSpec{manifest.DockerComposeTarget()}
	}

	if manifest.IsK8s() {
		return []model.TargetSpec{manifest.ImageTarget, manifest.K8sTarget()}
	}

	return nil
}

// Extract a set of build states from a manifest for BuildAndDeploy.
func buildStateSet(manifest model.Manifest, specs []model.TargetSpec, ms *store.ManifestState) store.BuildStateSet {
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

		// Kubernetes-based builds can update containers in-place.
		//
		// We don't want to pass along the kubernetes data if the pod is crashing,
		// because we're not confident that this state is accurate (due to how k8s
		// reschedules pods).
		if manifest.IsK8s() && !ms.NeedsRebuildFromCrash {
			buildState = buildState.WithDeployTarget(store.NewDeployInfo(ms.PodSet))
		}
		buildStateSet[id] = buildState
	}

	return buildStateSet
}

var _ store.Subscriber = &BuildController{}
