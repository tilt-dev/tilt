package configs

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type ConfigsController struct {
	disabledForTesting bool
	tfl                tiltfile.TiltfileLoader
	dockerClient       docker.Client
	clock              func() time.Time
}

func NewConfigsController(tfl tiltfile.TiltfileLoader, dockerClient docker.Client) *ConfigsController {
	return &ConfigsController{
		tfl:          tfl,
		dockerClient: dockerClient,
		clock:        time.Now,
	}
}

func (cc *ConfigsController) SetTiltfileLoaderForTesting(tfl tiltfile.TiltfileLoader) {
	cc.tfl = tfl
}

func (cc *ConfigsController) DisableForTesting(disabled bool) {
	cc.disabledForTesting = disabled
}

// Modeled after BuildController.nextTargetToBuild. Check to see that:
// 1) There's currently no Tiltfile build running,
// 2) There are pending file changes, and
// 3) Those files have changed since the last Tiltfile build
//    (so that we don't keep re-running a failed build)
func (cc *ConfigsController) shouldBuild(state store.EngineState) bool {
	isRunning := !state.TiltfileState.CurrentBuild.StartTime.IsZero()
	if isRunning {
		return false
	}

	for _, changeTime := range state.PendingConfigFileChanges {
		lastStartTime := state.TiltfileState.LastBuild().StartTime
		if changeTime.After(lastStartTime) {
			return true
		}
	}
	return false
}

func logTiltfileChanges(ctx context.Context, filesChanged map[string]bool) {
	var filenames []string
	for k := range filesChanged {
		filenames = append(filenames, k)
	}

	l := logger.Get(ctx)

	if len(filenames) > 0 {
		p := logger.Green(l).Sprintf("%d changed: ", len(filenames))
		l.Infof("\n%s%v\n", p, ospath.FormatFileChangeList(filenames))
	}
}

func (cc *ConfigsController) loadTiltfile(ctx context.Context, st store.RStore,
	initManifests []model.ManifestName, filesChanged map[string]bool, tiltfilePath string) {

	startTime := cc.clock()
	st.Dispatch(ConfigsReloadStartedAction{FilesChanged: filesChanged, StartTime: startTime})

	matching := map[string]bool{}
	for _, m := range initManifests {
		matching[string(m)] = true
	}

	actionWriter := NewTiltfileLogWriter(st)
	ctx = logger.WithLogger(ctx, logger.NewLogger(logger.Get(ctx).Level(), actionWriter))

	state := st.RLockState()
	globalLogLineCountAtExecStart := state.Log.LineCount()
	firstBuild := !state.TiltfileState.StartedFirstBuild()
	if !firstBuild {
		logTiltfileChanges(ctx, filesChanged)
	}
	st.RUnlockState()

	tlr := cc.tfl.Load(ctx, tiltfilePath, matching)
	if tlr.Error == nil && len(tlr.Manifests) == 0 {
		tlr.Error = fmt.Errorf("No resources found. Check out https://docs.tilt.dev/tutorial.html to get started!")
	}
	if tlr.Error != nil {
		logger.Get(ctx).Infof(tlr.Error.Error())
	}

	if tlr.Orchestrator() != model.OrchestratorUnknown {
		cc.dockerClient.SetOrchestrator(tlr.Orchestrator())
		dockerErr := cc.dockerClient.CheckConnected()
		if tlr.Error == nil && dockerErr != nil {
			tlr.Error = errors.Wrap(dockerErr, "Failed to connect to Docker")
		}
	}

	st.Dispatch(ConfigsReloadedAction{
		Manifests:                     tlr.Manifests,
		ConfigFiles:                   tlr.ConfigFiles,
		TiltIgnoreContents:            tlr.TiltIgnoreContents,
		FinishTime:                    cc.clock(),
		Err:                           tlr.Error,
		Warnings:                      tlr.Warnings,
		Features:                      tlr.FeatureFlags,
		TeamName:                      tlr.TeamName,
		Secrets:                       tlr.Secrets,
		DockerPruneSettings:           tlr.DockerPruneSettings,
		GlobalLogLineCountAtExecStart: globalLogLineCountAtExecStart,
	})
}

func (cc *ConfigsController) OnChange(ctx context.Context, st store.RStore) {
	if cc.disabledForTesting {
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()

	initManifests := state.InitManifests
	if !cc.shouldBuild(state) {
		return
	}

	filesChanged := make(map[string]bool)
	for k := range state.PendingConfigFileChanges {
		filesChanged[k] = true
	}

	tiltfilePath, err := state.RelativeTiltfilePath()
	if err != nil {
		st.Dispatch(store.NewErrorAction(err))
		return
	}

	// Release the state lock and load the tiltfile in a separate goroutine
	go cc.loadTiltfile(ctx, st, initManifests, filesChanged, tiltfilePath)
}
