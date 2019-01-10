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
	ctx          context.Context
	manifest     model.Manifest
	buildState   store.BuildState
	buildReason  model.BuildReason
	filesChanged []string
	firstBuild   bool
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

	filesChanged := make([]string, 0, len(ms.PendingFileChanges))
	for file, _ := range ms.PendingFileChanges {
		filesChanged = append(filesChanged, file)
	}
	sort.Strings(filesChanged)

	buildState := store.NewBuildState(ms.LastSuccessfulResult, filesChanged)

	if !ms.NeedsRebuildFromCrash {
		buildState = buildState.WithDeployTarget(store.NewDeployInfo(ms.PodSet))
	}

	buildReason := ms.NextBuildReason()

	// Send the logs to both the EngineState and the normal log stream.
	actionWriter := BuildLogActionWriter{
		store:        st,
		manifestName: manifest.Name,
	}
	ctx = logger.CtxWithForkedOutput(ctx, actionWriter)

	return buildEntry{
		ctx:          ctx,
		manifest:     manifest,
		firstBuild:   firstBuild,
		buildReason:  buildReason,
		buildState:   buildState,
		filesChanged: filesChanged,
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
		st.Dispatch(BuildStartedAction{
			ManifestName: entry.manifest.Name,
			StartTime:    time.Now(),
			FilesChanged: entry.filesChanged,
			Reason:       entry.buildReason,
		})
		c.logBuildEntry(entry.ctx, entry)
		result, err := c.b.BuildAndDeploy(entry.ctx, entry.manifest, entry.buildState)
		st.Dispatch(NewBuildCompleteAction(result, err))
	}()
}

func (c *BuildController) logBuildEntry(ctx context.Context, entry buildEntry) {
	firstBuild := entry.firstBuild
	manifest := entry.manifest
	buildState := entry.buildState

	l := logger.Get(ctx)
	if firstBuild {
		p := logger.Blue(l).Sprintf("\n──┤ Building: ")
		s := logger.Blue(l).Sprintf(" ├──────────────────────────────────────────────")
		l.Infof("%s%s%s", p, manifest.Name, s)
	} else {
		changedFiles := buildState.FilesChanged()
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
		l.Infof("%s%s%s", rp, manifest.Name, rs)
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

var _ store.Subscriber = &BuildController{}
