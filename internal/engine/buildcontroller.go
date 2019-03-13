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
	// Don't build anything if there are pending config file changes.
	// We want the Tiltfile to re-run first.
	if len(state.PendingConfigFileChanges) > 0 {
		return nil
	}

	// put no-build manifests first since they're more likely to be
	// 1. fast and 2. dependencies of other services (e.g., redis)
	targets := append([]*store.ManifestTarget{}, state.Targets()...)
	sort.Sort(newNoBuildsManifestsFirst(targets))

	// First, go through all the manifests in order.
	// If any of them haven't started yet, build them now.
	for _, mt := range targets {
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
	for _, mt := range targets {
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
		for _, mt := range targets {
			ok, newTime := mt.State.HasPendingChangesBefore(earliest)
			if ok {
				choice = mt
				earliest = newTime
			}
		}
	}

	return choice
}

type noBuildManifestsFirst struct {
	mts             []*store.ManifestTarget
	origIndexByName map[string]int
}

var _ sort.Interface = noBuildManifestsFirst{}

func newNoBuildsManifestsFirst(mts []*store.ManifestTarget) *noBuildManifestsFirst {
	indexByName := make(map[string]int)
	for i, mt := range mts {
		indexByName[mt.Manifest.Name.String()] = i
	}
	return &noBuildManifestsFirst{
		mts:             mts,
		origIndexByName: indexByName,
	}
}

func (nbmf noBuildManifestsFirst) Len() int {
	return len(nbmf.mts)
}

func (nbmf noBuildManifestsFirst) Less(i, j int) bool {
	isNoBuild := func(mt *store.ManifestTarget) bool {
		return len(mt.Manifest.ImageTargets) == 0
	}

	a := nbmf.mts[i]
	b := nbmf.mts[j]

	nba := isNoBuild(a)
	nbb := isNoBuild(b)

	if nba == nbb {
		return nbmf.origIndexByName[a.Manifest.Name.String()] < nbmf.origIndexByName[b.Manifest.Name.String()]
	} else {
		return nba
	}
}

func (nbmf noBuildManifestsFirst) Swap(i, j int) {
	nbmf.mts[i], nbmf.mts[j] = nbmf.mts[j], nbmf.mts[i]
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

func (c *BuildController) logBuildEntry(ctx context.Context, entry buildEntry, changedFiles []string) {
	firstBuild := entry.firstBuild
	name := entry.name

	l := logger.Get(ctx)
	if firstBuild {
		p := logger.Blue(l).Sprintf("──┤ Building: ")
		s := logger.Blue(l).Sprintf(" ├──────────────────────────────────────────────")
		l.Infof("\n%s%s%s", p, name, s)
	} else {
		var changedPathsToPrint []string
		if len(changedFiles) > maxChangedFilesToPrint {
			changedPathsToPrint = append(changedPathsToPrint, changedFiles[:maxChangedFilesToPrint]...)
			changedPathsToPrint = append(changedPathsToPrint, "...")
		} else {
			changedPathsToPrint = changedFiles
		}

		if len(changedFiles) > 0 {
			p := logger.Green(l).Sprintf("%d changed: ", len(changedFiles))
			l.Infof("\n%s%v\n", p, ospath.TryAsCwdChildren(changedPathsToPrint))
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
		ManifestName: w.manifestName,
		logEvent:     newLogEvent(append([]byte{}, p...)),
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
	}

	return result
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
					buildState = buildState.WithDeployTarget(store.NewDeployInfo(iTarget, ms.PodSet))
				}

				if manifest.IsDC() {
					buildState = buildState.WithDeployTarget(store.NewDeployInfoFromDC(ms.DCResourceState()))
				}
			}
		}
		buildStateSet[id] = buildState
	}

	return buildStateSet
}

var _ store.Subscriber = &BuildController{}
