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
	ctx               context.Context
	manifest          model.Manifest
	buildState        store.BuildState
	filesChanged      []string
	firstBuild        bool
	needsConfigReload bool
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

	if len(state.ManifestsToBuild) == 0 {
		return buildEntry{}, false
	}

	if c.lastActionCount == state.BuildControllerActionCount {
		return buildEntry{}, false
	}

	mn := state.ManifestsToBuild[0]
	c.lastActionCount = state.BuildControllerActionCount
	ms := state.ManifestStates[mn]
	manifest := ms.Manifest
	firstBuild := !ms.HasBeenBuilt

	filesChanged := make([]string, 0, len(ms.PendingFileChanges))
	for file, _ := range ms.PendingFileChanges {
		filesChanged = append(filesChanged, file)
	}
	sort.Strings(filesChanged)

	buildState := store.NewBuildState(ms.LastBuild, filesChanged)

	needsConfigReload := ms.ConfigIsDirty

	// TODO(nick): This is...not great, because it modifies the build log in place.
	// A better solution would dispatch actions (like PodLogManager does) so that
	// they go thru the state loop and immediately update in the UX.
	ctx = logger.CtxWithForkedOutput(ctx, ms.CurrentBuildLog)

	return buildEntry{
		ctx:               ctx,
		manifest:          manifest,
		firstBuild:        firstBuild,
		buildState:        buildState,
		filesChanged:      filesChanged,
		needsConfigReload: needsConfigReload,
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
		// 	if entry.needsConfigReload {
		// 		newManifest, newGlobalYAML, err := getNewManifestFromTiltfile(entry.ctx, entry.manifest.Name)
		// 		st.Dispatch(GlobalYAMLManifestReloadedAction{
		// 			GlobalYAML: newGlobalYAML,
		// 		})
		// 		st.Dispatch(ManifestReloadedAction{
		// 			OldManifest: entry.manifest,
		// 			NewManifest: newManifest,
		// 			Error:       err,
		// 		})
		// 		return
		// 	}

		st.Dispatch(BuildStartedAction{
			Manifest:     entry.manifest,
			StartTime:    time.Now(),
			FilesChanged: entry.filesChanged,
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

// func getNewManifestFromTiltfile(ctx context.Context, name model.ManifestName) (model.Manifest, model.YAMLManifest, error) {
// 	// Sends any output to the CurrentBuildLog
// 	t, err := tiltfile.Load(ctx, tiltfile.FileName)
// 	if err != nil {
// 		return model.Manifest{}, model.YAMLManifest{}, err
// 	}
// 	newManifests, globalYAML, err := t.GetManifestConfigsAndGlobalYAML(ctx, name)
// 	if err != nil {
// 		return model.Manifest{}, model.YAMLManifest{}, err
// 	}
// 	if len(newManifests) != 1 {
// 		return model.Manifest{}, model.YAMLManifest{}, fmt.Errorf("Expected there to be 1 manifest for %s, got %d", name, len(newManifests))
// 	}
// 	newManifest := newManifests[0]

// 	return newManifest, globalYAML, nil
// }

// func getNewManifestsFromTiltfile(ctx context.Context, names []model.ManifestName) ([]model.Manifest, model.YAMLManifest, error) {
// 	// Sends any output to the CurrentBuildLog
// 	t, err := tiltfile.Load(ctx, tiltfile.FileName)
// 	if err != nil {
// 		return []model.Manifest{}, model.YAMLManifest{}, err
// 	}
// 	newManifests, globalYAML, err := t.GetManifestConfigsAndGlobalYAML(ctx, names...)
// 	if err != nil {
// 		return []model.Manifest{}, model.YAMLManifest{}, err
// 	}

// 	return newManifests, globalYAML, nil
// }

var _ store.Subscriber = &BuildController{}
