package configs

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/core/tiltfile"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ConfigsController struct {
	disabledForTesting bool
	tfl                tiltfile.TiltfileLoader
	dockerClient       docker.Client
	clock              func() time.Time
	ctrlClient         ctrlclient.Client
	buildSource        *ctrltiltfile.BuildSource
	loadStartedCount   int // used to synchronize with state
}

func NewConfigsController(tfl tiltfile.TiltfileLoader, dockerClient docker.Client, ctrlClient ctrlclient.Client, buildSource *ctrltiltfile.BuildSource) *ConfigsController {
	return &ConfigsController{
		tfl:          tfl,
		dockerClient: dockerClient,
		ctrlClient:   ctrlClient,
		clock:        time.Now,
		buildSource:  buildSource,
	}
}

func (cc *ConfigsController) SetTiltfileLoaderForTesting(tfl tiltfile.TiltfileLoader) {
	cc.tfl = tfl
}

func (cc *ConfigsController) DisableForTesting(disabled bool) {
	cc.disabledForTesting = disabled
}

// Modeled after BuildController.needsBuild and NextBuildReason(). Check to see that:
// 1) There's currently no Tiltfile build running,
// 2) There are pending file changes, and
// 3) Those files have changed since the last Tiltfile build
//    (so that we don't keep re-running a failed build)
// 4) OR the command-line args have changed since the last Tiltfile build
// 5) OR user has manually triggered a Tiltfile build
func (cc *ConfigsController) needsBuild(ctx context.Context, st store.RStore) (*ctrltiltfile.BuildEntry, bool) {
	state := st.RLockState()
	defer st.RUnlockState()

	// Don't start the next build until the previous action has been recorded,
	// so that we don't accidentally repeat the same build.
	if cc.loadStartedCount != state.StartedTiltfileLoadCount {
		return nil, false
	}

	// Don't start the next build if the last completion hasn't been recorded yet.
	for _, ms := range state.TiltfileStates {
		isRunning := !ms.CurrentBuild.StartTime.IsZero()
		if isRunning {
			return nil, false
		}
	}

	for _, name := range state.TiltfileDefinitionOrder {
		tfState, ok := state.TiltfileStates[name]
		if !ok {
			continue
		}

		var reason model.BuildReason
		lastStartTime := tfState.LastBuild().StartTime
		if !tfState.StartedFirstBuild() {
			reason = reason.With(model.BuildReasonFlagInit)
		}

		hasPendingChanges, _ := tfState.HasPendingChanges()
		if hasPendingChanges {
			reason = reason.With(model.BuildReasonFlagChangedFiles)
		}

		if state.UserConfigState.ArgsChangeTime.After(lastStartTime) {
			reason = reason.With(model.BuildReasonFlagTiltfileArgs)
		}

		if state.TiltfileInTriggerQueue() {
			reason = reason.With(tfState.TriggerReason)
		}

		if reason == model.BuildReasonNone {
			continue
		}

		filesChanged := []string{}
		for _, st := range tfState.BuildStatuses {
			for k := range st.PendingFileChanges {
				filesChanged = append(filesChanged, k)
			}
		}
		filesChanged = sliceutils.DedupedAndSorted(filesChanged)

		tiltfilePath, err := state.RelativeTiltfilePath()
		if err != nil {
			st.Dispatch(store.NewErrorAction(err))
		}

		cc.loadStartedCount++

		return &ctrltiltfile.BuildEntry{
			Name:                  name,
			FilesChanged:          filesChanged,
			BuildReason:           reason,
			UserConfigState:       state.UserConfigState,
			TiltfilePath:          tiltfilePath,
			CheckpointAtExecStart: state.LogStore.Checkpoint(),
			EngineMode:            state.EngineMode,
		}, true
	}

	return nil, false
}

func (cc *ConfigsController) loadTiltfile(ctx context.Context, st store.RStore, entry *ctrltiltfile.BuildEntry) {
	startTime := cc.clock()
	st.Dispatch(ConfigsReloadStartedAction{
		Name:         entry.Name,
		FilesChanged: entry.FilesChanged,
		StartTime:    startTime,
		SpanID:       SpanIDForLoadCount(cc.loadStartedCount),
		Reason:       entry.BuildReason,
	})

	actionWriter := NewTiltfileLogWriter(entry.Name, st, cc.loadStartedCount)
	ctx = logger.CtxWithLogHandler(ctx, actionWriter)

	buildcontrol.LogBuildEntry(ctx, buildcontrol.BuildEntry{
		Name:         entry.Name,
		BuildReason:  entry.BuildReason,
		FilesChanged: entry.FilesChanged,
	})

	userConfigState := entry.UserConfigState
	if entry.BuildReason.Has(model.BuildReasonFlagTiltfileArgs) {
		logger.Get(ctx).Infof("Tiltfile args changed to: %v", userConfigState.Args)
	}

	tlr := cc.tfl.Load(ctx, entry.TiltfilePath, userConfigState)
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

	// TODO(nick): Rewrite to handle multiple tiltfiles.
	err := updateOwnedObjects(ctx, cc.ctrlClient, tlr, entry.EngineMode)
	if err != nil {
		if tlr.Error == nil {
			tlr.Error = errors.Wrap(err, "Failed to update API server")
		} else {
			logger.Get(ctx).Errorf("Failed to update API server: %v", err)
		}
	}

	if tlr.Error != nil {
		logger.Get(ctx).Errorf("%s", tlr.Error.Error())
	}

	st.Dispatch(ConfigsReloadedAction{
		Name:                  entry.Name,
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
		CheckpointAtExecStart: entry.CheckpointAtExecStart,
		VersionSettings:       tlr.VersionSettings,
		UpdateSettings:        tlr.UpdateSettings,
		WatchSettings:         tlr.WatchSettings,
	})
}

func (cc *ConfigsController) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) error {
	if cc.disabledForTesting {
		return nil
	}

	entry, ok := cc.needsBuild(ctx, st)
	if !ok {
		return nil
	}

	cc.buildSource.SetEntry(entry)
	cc.loadTiltfile(ctx, st, entry)
	return nil
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
