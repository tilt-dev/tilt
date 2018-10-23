package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
)

type BuildController struct {
	b                       BuildAndDeployer
	lastCompletedBuildCount int
}

type buildEntry struct {
	ctx        context.Context
	manifest   model.Manifest
	buildState store.BuildState
	firstBuild bool
}

func NewBuildController(b BuildAndDeployer) *BuildController {
	return &BuildController{
		b: b,
		lastCompletedBuildCount: -1,
	}
}

func (c *BuildController) needsBuild(ctx context.Context, st *store.Store) (buildEntry, bool) {
	state := st.RLockState()
	defer st.RUnlockState()

	if state.CurrentlyBuilding == "" {
		return buildEntry{}, false
	}

	if c.lastCompletedBuildCount == state.CompletedBuildCount {
		return buildEntry{}, false
	}

	c.lastCompletedBuildCount = state.CompletedBuildCount
	ms := state.ManifestStates[state.CurrentlyBuilding]
	manifest := ms.Manifest
	firstBuild := !ms.HasBeenBuilt

	mountedFilesChangedSinceLastSuccessfulBuild, err := ms.WithoutUnmountedConfigFiles(ctx, ms.FileChangesSinceLastSuccessfulBuild)
	if err != nil {
		logger.Get(ctx).Infof("error determining whether files are unmounted config files: %v", err)
		return buildEntry{}, false
	}

	buildState := store.NewBuildState(ms.LastBuild, mountedFilesChangedSinceLastSuccessfulBuild)

	// TODO(nick): This is...not great, because it modifies the build log in place.
	// A better solution would dispatch actions (like PodLogManager does) so that
	// they go thru the state loop and immediately update in the UX.
	ctx = logger.CtxWithForkedOutput(ctx, ms.CurrentBuildLog)

	return buildEntry{
		ctx:        ctx,
		manifest:   manifest,
		firstBuild: firstBuild,
		buildState: buildState,
	}, true
}

func (c *BuildController) OnChange(ctx context.Context, st *store.Store) {
	entry, ok := c.needsBuild(ctx, st)
	if !ok {
		return
	}

	go func() {
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

		p := logger.Green(l).Sprintf("\n%d changed: ", len(changedFiles))
		l.Infof("%s%v\n", p, ospath.TryAsCwdChildren(changedPathsToPrint))
		rp := logger.Blue(l).Sprintf("──┤ Rebuilding: ")
		rs := logger.Blue(l).Sprintf(" ├────────────────────────────────────────────")
		l.Infof("%s%s%s", rp, manifest.Name, rs)
	}
}

var _ store.Subscriber = &BuildController{}
