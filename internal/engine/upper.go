package engine

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/tilt-dev/wmclient/pkg/analytics"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch"
	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/core/tiltfile"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/engine/dcwatch"
	"github.com/tilt-dev/tilt/internal/engine/k8swatch"
	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/engine/session"
	"github.com/tilt-dev/tilt/internal/hud"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/buildcontrols"
	"github.com/tilt-dev/tilt/internal/store/clusters"
	"github.com/tilt-dev/tilt/internal/store/configmaps"
	"github.com/tilt-dev/tilt/internal/store/filewatches"
	"github.com/tilt-dev/tilt/internal/store/kubernetesapplys"
	"github.com/tilt-dev/tilt/internal/store/kubernetesdiscoverys"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/internal/store/tiltfiles"
	"github.com/tilt-dev/tilt/internal/store/uiresources"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// TODO(nick): maybe this should be called 'BuildEngine' or something?
// Upper seems like a poor and undescriptive name.
type Upper struct {
	store *store.Store
}

type ServiceWatcherMaker func(context.Context, *store.Store) error
type PodWatcherMaker func(context.Context, *store.Store) error

func NewUpper(ctx context.Context, st *store.Store, subs []store.Subscriber) (Upper, error) {
	// There's not really a good reason to add all the subscribers
	// in NewUpper(), but it's as good a place as any.
	for _, sub := range subs {
		err := st.AddSubscriber(ctx, sub)
		if err != nil {
			return Upper{}, err
		}
	}

	return Upper{
		store: st,
	}, nil
}

func (u Upper) Dispatch(action store.Action) {
	u.store.Dispatch(action)
}

func (u Upper) Start(
	ctx context.Context,
	args []string,
	b model.TiltBuild,
	fileName string,
	initTerminalMode store.TerminalMode,
	analyticsUserOpt analytics.Opt,
	token token.Token,
	cloudAddress string,
) error {

	startTime := time.Now()

	absTfPath, err := filepath.Abs(fileName)
	if err != nil {
		return err
	}

	configFiles := []string{absTfPath}

	return u.Init(ctx, InitAction{
		TiltfilePath:     absTfPath,
		ConfigFiles:      configFiles,
		UserArgs:         args,
		TiltBuild:        b,
		StartTime:        startTime,
		AnalyticsUserOpt: analyticsUserOpt,
		Token:            token,
		CloudAddress:     cloudAddress,
		TerminalMode:     initTerminalMode,
	})
}

func (u Upper) Init(ctx context.Context, action InitAction) error {
	u.store.Dispatch(action)
	return u.store.Loop(ctx)
}

func upperReducerFn(ctx context.Context, state *store.EngineState, action store.Action) {
	// Allow exitAction and dumpEngineStateAction even if there's a fatal error
	if exitAction, isExitAction := action.(hud.ExitAction); isExitAction {
		handleHudExitAction(state, exitAction)
		return
	}
	if _, isDumpEngineStateAction := action.(hud.DumpEngineStateAction); isDumpEngineStateAction {
		handleDumpEngineStateAction(ctx, state)
		return
	}

	if state.FatalError != nil {
		return
	}

	switch action := action.(type) {
	case InitAction:
		handleInitAction(ctx, state, action)
	case store.ErrorAction:
		state.FatalError = action.Error
	case hud.ExitAction:
		handleHudExitAction(state, action)

	// TODO(nick): Delete these handlers in favor of the bog-standard ones that copy
	// the api models directly.
	case filewatch.FileWatchUpdateStatusAction:
		filewatch.HandleFileWatchUpdateStatusEvent(ctx, state, action)

	case k8swatch.ServiceChangeAction:
		handleServiceEvent(ctx, state, action)
	case store.K8sEventAction:
		handleK8sEvent(ctx, state, action)
	case buildcontrols.BuildCompleteAction:
		buildcontrols.HandleBuildCompleted(ctx, state, action)
	case buildcontrols.BuildStartedAction:
		buildcontrols.HandleBuildStarted(ctx, state, action)
	case ctrltiltfile.ConfigsReloadStartedAction:
		ctrltiltfile.HandleConfigsReloadStarted(ctx, state, action)
	case ctrltiltfile.ConfigsReloadedAction:
		ctrltiltfile.HandleConfigsReloaded(ctx, state, action)
	case dcwatch.EventAction:
		handleDockerComposeEvent(ctx, state, action)
	case server.AppendToTriggerQueueAction:
		state.AppendToTriggerQueue(action.Name, action.Reason)
	case hud.DumpEngineStateAction:
		handleDumpEngineStateAction(ctx, state)
	case store.AnalyticsUserOptAction:
		handleAnalyticsUserOptAction(state, action)
	case store.AnalyticsNudgeSurfacedAction:
		handleAnalyticsNudgeSurfacedAction(ctx, state)
	case store.TiltCloudStatusReceivedAction:
		handleTiltCloudStatusReceivedAction(state, action)
	case store.UserStartedTiltCloudRegistrationAction:
		handleUserStartedTiltCloudRegistrationAction(state)
	case store.PanicAction:
		handlePanicAction(state, action)
	case store.LogAction:
		handleLogAction(state, action)
	case session.SessionUpdateStatusAction:
		session.HandleSessionUpdateStatusAction(state, action)
	case prompt.SwitchTerminalModeAction:
		handleSwitchTerminalModeAction(state, action)
	case server.OverrideTriggerModeAction:
		handleOverrideTriggerModeAction(ctx, state, action)
	case local.CmdCreateAction:
		local.HandleCmdCreateAction(state, action)
	case local.CmdUpdateStatusAction:
		local.HandleCmdUpdateStatusAction(state, action)
	case local.CmdDeleteAction:
		local.HandleCmdDeleteAction(state, action)
	case tiltfiles.TiltfileUpsertAction:
		tiltfiles.HandleTiltfileUpsertAction(state, action)
	case tiltfiles.TiltfileDeleteAction:
		tiltfiles.HandleTiltfileDeleteAction(state, action)
	case filewatches.FileWatchUpsertAction:
		filewatches.HandleFileWatchUpsertAction(state, action)
	case filewatches.FileWatchDeleteAction:
		filewatches.HandleFileWatchDeleteAction(state, action)
	case kubernetesapplys.KubernetesApplyUpsertAction:
		kubernetesapplys.HandleKubernetesApplyUpsertAction(state, action)
	case kubernetesapplys.KubernetesApplyDeleteAction:
		kubernetesapplys.HandleKubernetesApplyDeleteAction(state, action)
	case kubernetesdiscoverys.KubernetesDiscoveryUpsertAction:
		kubernetesdiscoverys.HandleKubernetesDiscoveryUpsertAction(state, action)
	case kubernetesdiscoverys.KubernetesDiscoveryDeleteAction:
		kubernetesdiscoverys.HandleKubernetesDiscoveryDeleteAction(state, action)
	case uiresources.UIResourceUpsertAction:
		uiresources.HandleUIResourceUpsertAction(state, action)
	case uiresources.UIResourceDeleteAction:
		uiresources.HandleUIResourceDeleteAction(state, action)
	case configmaps.ConfigMapUpsertAction:
		configmaps.HandleConfigMapUpsertAction(state, action)
	case configmaps.ConfigMapDeleteAction:
		configmaps.HandleConfigMapDeleteAction(state, action)
	case liveupdates.LiveUpdateUpsertAction:
		liveupdates.HandleLiveUpdateUpsertAction(state, action)
	case liveupdates.LiveUpdateDeleteAction:
		liveupdates.HandleLiveUpdateDeleteAction(state, action)
	case clusters.ClusterUpsertAction:
		clusters.HandleClusterUpsertAction(state, action)
	case clusters.ClusterDeleteAction:
		clusters.HandleClusterDeleteAction(state, action)
	default:
		state.FatalError = fmt.Errorf("unrecognized action: %T", action)
	}
}

var UpperReducer = store.Reducer(upperReducerFn)

func handleLogAction(state *store.EngineState, action store.LogAction) {
	state.LogStore.Append(action, state.Secrets)
}

func handleSwitchTerminalModeAction(state *store.EngineState, action prompt.SwitchTerminalModeAction) {
	state.TerminalMode = action.Mode
}

func handleServiceEvent(ctx context.Context, state *store.EngineState, action k8swatch.ServiceChangeAction) {
	service := action.Service
	ms, ok := state.ManifestState(action.ManifestName)
	if !ok {
		return
	}

	runtime := ms.K8sRuntimeState()
	runtime.LBs[k8s.ServiceName(service.Name)] = action.URL
}

func handleK8sEvent(ctx context.Context, state *store.EngineState, action store.K8sEventAction) {
	// TODO(nick): I think we whould so something more intelligent here, where we
	// have special treatment for different types of events, e.g.:
	//
	// - Attach Image Pulling/Pulled events to the pod state, and display how much
	//   time elapsed between them.
	// - Display Node unready events as part of a health indicator, and display how
	//   long it takes them to resolve.
	handleLogAction(state, action.ToLogAction(action.ManifestName))
}

func handleDumpEngineStateAction(ctx context.Context, engineState *store.EngineState) {
	f, err := ioutil.TempFile("", "tilt-engine-state-*.txt")
	if err != nil {
		logger.Get(ctx).Infof("error creating temp file to write engine state: %v", err)
		return
	}

	logger.Get(ctx).Infof("dumped tilt engine state to %q", f.Name())
	spew.Fdump(f, engineState)

	err = f.Close()
	if err != nil {
		logger.Get(ctx).Infof("error closing engine state temp file: %v", err)
		return
	}
}

func handleInitAction(ctx context.Context, engineState *store.EngineState, action InitAction) {
	engineState.TiltBuildInfo = action.TiltBuild
	engineState.TiltStartTime = action.StartTime
	engineState.DesiredTiltfilePath = action.TiltfilePath
	engineState.TiltfileConfigPaths[model.MainTiltfileManifestName] = action.ConfigFiles
	engineState.UserConfigState = model.NewUserConfigState(action.UserArgs)
	engineState.AnalyticsUserOpt = action.AnalyticsUserOpt
	engineState.CloudAddress = action.CloudAddress
	engineState.Token = action.Token
	engineState.TerminalMode = action.TerminalMode
}

func handleHudExitAction(state *store.EngineState, action hud.ExitAction) {
	if action.Err != nil {
		state.FatalError = action.Err
	} else {
		state.UserExited = true
	}
}

func handlePanicAction(state *store.EngineState, action store.PanicAction) {
	state.PanicExited = action.Err
}

func handleDockerComposeEvent(ctx context.Context, engineState *store.EngineState, action dcwatch.EventAction) {
	evt := action.Event
	mn := model.ManifestName(evt.Service)
	ms, ok := engineState.ManifestState(mn)
	if !ok {
		// No corresponding manifest, nothing to do
		return
	}

	state, _ := ms.RuntimeState.(dockercompose.State)

	state = state.WithContainerID(container.ID(evt.ID)).
		WithSpanID(runtimelog.SpanIDForDCService(mn)).
		WithContainerState(action.ContainerState)

	if evt.IsStartupEvent() {
		state = state.WithStartTime(action.Time)
		// NB: this will differ from StartTime once we support DC health checks
		state = state.WithLastReadyTime(action.Time)
	}

	ms.RuntimeState = state
}

func handleAnalyticsUserOptAction(state *store.EngineState, action store.AnalyticsUserOptAction) {
	state.AnalyticsUserOpt = action.Opt
}

// The first time we hear that the analytics nudge was surfaced, record a metric.
// We double check !state.AnalyticsNudgeSurfaced -- i.e. that the state doesn't
// yet know that we've surfaced the nudge -- to ensure that we only record this
// metric once (since it's an anonymous metric, we can't slice it by e.g. # unique
// users, so the numbers need to be as accurate as possible).
func handleAnalyticsNudgeSurfacedAction(ctx context.Context, state *store.EngineState) {
	if !state.AnalyticsNudgeSurfaced {
		tiltanalytics.Get(ctx).Incr("analytics.nudge.surfaced", nil)
		state.AnalyticsNudgeSurfaced = true
	}
}

func handleTiltCloudStatusReceivedAction(state *store.EngineState, action store.TiltCloudStatusReceivedAction) {
	if action.IsPostRegistrationLookup {
		state.CloudStatus.WaitingForStatusPostRegistration = false
	}
	if !action.Found {
		state.CloudStatus.TokenKnownUnregistered = true
		state.CloudStatus.Username = ""
	} else {
		state.CloudStatus.TokenKnownUnregistered = false
		state.CloudStatus.Username = action.Username
		state.CloudStatus.TeamName = action.TeamName
	}

	state.SuggestedTiltVersion = action.SuggestedTiltVersion
}

func handleUserStartedTiltCloudRegistrationAction(state *store.EngineState) {
	state.CloudStatus.WaitingForStatusPostRegistration = true
}

func handleOverrideTriggerModeAction(ctx context.Context, state *store.EngineState,
	action server.OverrideTriggerModeAction) {
	// TODO(maia): in this implementation, overrides do NOT persist across Tiltfile loads
	//   (i.e. the next Tiltfile load will wipe out the override we just put in place).
	//   If we want to keep this functionality, the next step is to store the set of overrides
	//   on the engine state, and whenever we load the manifest from the Tiltfile, apply
	//   any necessary overrides.

	// We validate trigger mode when we receive a request, so this should never happen
	if !model.ValidTriggerMode(action.TriggerMode) {
		logger.Get(ctx).Errorf("INTERNAL ERROR overriding trigger mode: invalid trigger mode %d", action.TriggerMode)
		return
	}

	for _, mName := range action.ManifestNames {
		mt, ok := state.ManifestTargets[mName]
		if !ok {
			// We validate manifest names when we receive a request, so this should never happen
			logger.Get(ctx).Errorf("INTERNAL ERROR overriding trigger mode: no such manifest %q", mName)
			return
		}
		mt.Manifest.TriggerMode = action.TriggerMode
	}
}
