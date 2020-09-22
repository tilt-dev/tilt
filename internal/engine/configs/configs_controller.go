package configs

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type ConfigsController struct {
	disabledForTesting bool
	tfl                tiltfile.TiltfileLoader
	dockerClient       docker.Client
	clock              func() time.Time
	loadStartedCount   int // used to synchronize with state
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

type buildEntry struct {
	filesChanged          []string
	buildReason           model.BuildReason
	userConfigState       model.UserConfigState
	tiltfilePath          string
	checkpointAtExecStart logstore.Checkpoint
}

func (e buildEntry) Name() model.ManifestName       { return model.TiltfileManifestName }
func (e buildEntry) FilesChanged() []string         { return e.filesChanged }
func (e buildEntry) BuildReason() model.BuildReason { return e.buildReason }

// Modeled after BuildController.needsBuild and NextBuildReason(). Check to see that:
// 1) There's currently no Tiltfile build running,
// 2) There are pending file changes, and
// 3) Those files have changed since the last Tiltfile build
//    (so that we don't keep re-running a failed build)
// 4) OR the command-line args have changed since the last Tiltfile build
func (cc *ConfigsController) needsBuild(ctx context.Context, st store.RStore) (buildEntry, bool) {
	state := st.RLockState()
	defer st.RUnlockState()

	// Don't start the next build until the previous action has been recorded,
	// so that we don't accidentally repeat the same build.
	if cc.loadStartedCount != state.StartedTiltfileLoadCount {
		return buildEntry{}, false
	}

	// Don't start the next build if the last completion hasn't been recorded yet.
	isRunning := !state.TiltfileState.CurrentBuild.StartTime.IsZero()
	if isRunning {
		return buildEntry{}, false
	}

	tfState := state.TiltfileState
	reason := tfState.TriggerReason
	lastStartTime := tfState.LastBuild().StartTime
	if !tfState.StartedFirstBuild() {
		reason = reason.With(model.BuildReasonFlagInit)
	}

	for _, changeTime := range state.PendingConfigFileChanges {
		if changeTime.After(lastStartTime) {
			reason = reason.With(model.BuildReasonFlagChangedFiles)
		}
	}

	if state.UserConfigState.ArgsChangeTime.After(lastStartTime) {
		reason = reason.With(model.BuildReasonFlagTiltfileArgs)
	}

	if reason == model.BuildReasonNone {
		return buildEntry{}, false
	}

	filesChanged := make([]string, 0, len(state.PendingConfigFileChanges))
	for k := range state.PendingConfigFileChanges {
		filesChanged = append(filesChanged, k)
	}
	filesChanged = sliceutils.DedupedAndSorted(filesChanged)

	tiltfilePath, err := state.RelativeTiltfilePath()
	if err != nil {
		st.Dispatch(store.NewErrorAction(err))
	}

	cc.loadStartedCount++

	return buildEntry{
		filesChanged:          filesChanged,
		buildReason:           reason,
		userConfigState:       state.UserConfigState,
		tiltfilePath:          tiltfilePath,
		checkpointAtExecStart: state.LogStore.Checkpoint(),
	}, true
}

func (cc *ConfigsController) loadTiltfile(ctx context.Context, st store.RStore, entry buildEntry) {
	startTime := cc.clock()
	st.Dispatch(ConfigsReloadStartedAction{
		FilesChanged: entry.filesChanged,
		StartTime:    startTime,
		SpanID:       SpanIDForLoadCount(cc.loadStartedCount),
		Reason:       entry.BuildReason(),
	})

	actionWriter := NewTiltfileLogWriter(st, cc.loadStartedCount)
	ctx = logger.CtxWithLogHandler(ctx, actionWriter)

	buildcontrol.LogBuildEntry(ctx, entry)

	userConfigState := entry.userConfigState
	if entry.BuildReason().Has(model.BuildReasonFlagTiltfileArgs) {
		logger.Get(ctx).Infof("Tiltfile args changed to: %v", userConfigState.Args)
	}

	tlr := cc.tfl.Load(ctx, entry.tiltfilePath, userConfigState)
	if tlr.Error == nil && len(tlr.Manifests) == 0 {
		tlr.Error = fmt.Errorf("No resources found. Check out https://docs.tilt.dev/tutorial.html to get started!")
	}

	if tlr.Orchestrator() != model.OrchestratorUnknown {
		cc.dockerClient.SetOrchestrator(tlr.Orchestrator())
	}

	if requiresDocker(tlr) {
		dockerErr := cc.dockerClient.CheckConnected()
		if tlr.Error == nil && dockerErr != nil {
			tlr.Error = errors.Wrap(dockerErr, "Failed to connect to Docker")
		}
	}

	if tlr.Error != nil {
		logger.Get(ctx).Infof("%s", tlr.Error.Error())
	}

	st.Dispatch(ConfigsReloadedAction{
		Manifests:             tlr.Manifests,
		Tiltignore:            tlr.Tiltignore,
		ConfigFiles:           tlr.ConfigFiles,
		FinishTime:            cc.clock(),
		Err:                   tlr.Error,
		Features:              tlr.FeatureFlags,
		TeamID:                tlr.TeamID,
		TelemetrySettings:     tlr.TelemetrySettings,
		MetricsSettings:       tlr.MetricsSettings,
		Secrets:               tlr.Secrets,
		AnalyticsTiltfileOpt:  tlr.AnalyticsOpt,
		DockerPruneSettings:   tlr.DockerPruneSettings,
		CheckpointAtExecStart: entry.checkpointAtExecStart,
		VersionSettings:       tlr.VersionSettings,
		UpdateSettings:        tlr.UpdateSettings,
		WatchSettings:         tlr.WatchSettings,
	})
}

func (cc *ConfigsController) OnChange(ctx context.Context, st store.RStore) {
	if cc.disabledForTesting {
		return
	}

	entry, ok := cc.needsBuild(ctx, st)
	if !ok {
		return
	}

	cc.loadTiltfile(ctx, st, entry)
}

func requiresDocker(tlr tiltfile.TiltfileLoadResult) bool {
	if tlr.Orchestrator() == model.OrchestratorDC {
		return true
	}

	for _, m := range tlr.Manifests {
		for _, iTarget := range m.ImageTargets {
			if iTarget.IsDockerBuild() {
				return true
			}
		}
	}

	return false
}
