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
	buildReason  store.BuildReason
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
func nextManifestToBuild(state store.EngineState) model.ManifestName {
	// First, go through all the manifests in order.
	// If any of them haven't started yet, build them now.
	for _, mn := range state.ManifestDefinitionOrder {
		ms, ok := state.ManifestStates[mn]
		if !ok {
			continue
		}
		if ms.Manifest.IsTiltfile {
			continue
		}
		if !ms.StartedFirstBuild {
			return mn
		}
	}

	// Next go through all the manifests, and check:
	// 1) all pending file changes, and
	// 2) all pending manifest changes
	// The earliest one is the one we want.
	choiceName := model.ManifestName("")
	earliest := time.Now()

	// always use a stable iteration order
	for _, mn := range state.ManifestDefinitionOrder {
		ms, ok := state.ManifestStates[mn]
		if !ok {
			continue
		}

		if ms.Manifest.IsTiltfile {
			continue
		}

		// Always prioritize builds that crashes and have an out-of-sync.
		if ms.NeedsRebuildFromCrash {
			return mn
		}

		t := ms.PendingManifestChange
		if t.Before(earliest) && ms.IsPendingTime(t) {
			choiceName = mn
			earliest = t
		}

		spurious, _ := onlySpuriousChanges(ms.PendingFileChanges)
		if !spurious {
			for _, t := range ms.PendingFileChanges {
				if t.Before(earliest) && ms.IsPendingTime(t) {
					choiceName = mn
					earliest = t
				}
			}
		}
	}

	return choiceName
}

func (c *BuildController) needsBuild(ctx context.Context, st store.RStore) (buildEntry, bool) {
	state := st.RLockState()
	defer st.RUnlockState()

	// Don't start the next build until the previous action has been recorded,
	// so that we don't accidentally repeat the same build.
	if c.lastActionCount == state.BuildControllerActionCount {
		return buildEntry{}, false
	}

	mn := nextManifestToBuild(state)
	if mn == "" {
		return buildEntry{}, false
	}

	c.lastActionCount = state.BuildControllerActionCount
	ms := state.ManifestStates[mn]
	manifest := ms.Manifest
	firstBuild := !ms.StartedFirstBuild

	filesChanged := make([]string, 0, len(ms.PendingFileChanges))
	for file, _ := range ms.PendingFileChanges {
		filesChanged = append(filesChanged, file)
	}
	sort.Strings(filesChanged)

	buildState := store.NewBuildState(ms.LastBuild, filesChanged)

	if !ms.NeedsRebuildFromCrash {
		buildState = buildState.WithDeployInfo(store.NewDeployInfo(ms.PodSet))
	}

	buildReason := ms.NextBuildReason()

	// Send the logs to both the EngineState and the normal log stream.
	actionWriter := BuildLogActionWriter{
		store:        st,
		manifestName: mn,
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
			Manifest:     entry.manifest,
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
		p := logger.Blue(l).Sprintf("──┤ Building: ")
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

		rp := logger.Blue(l).Sprintf("──┤ Rebuilding: ")
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
