package engine

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/windmilleng/wmclient/pkg/analytics"
	v1 "k8s.io/api/core/v1"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/engine/configs"
	"github.com/windmilleng/tilt/internal/engine/k8swatch"
	"github.com/windmilleng/tilt/internal/engine/runtimelog"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/token"
	"github.com/windmilleng/tilt/internal/watch"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

// When we see a file change, wait this long to see if any other files have changed, and bundle all changes together.
// 200ms is not the result of any kind of research or experimentation
// it might end up being a significant part of deployment delay, if we get the total latency <2s
// it might also be long enough that it misses some changes if the user has some operation involving a large file
//   (e.g., a binary dependency in git), but that's hopefully less of a problem since we'd get it in the next build
const watchBufferMinRestInMs = 200

// When waiting for a `watchBufferDurationInMs`-long break in file modifications to aggregate notifications,
// if we haven't seen a break by the time `watchBufferMaxTimeInMs` has passed, just send off whatever we've got
const watchBufferMaxTimeInMs = 10000

var watchBufferMinRestDuration = watchBufferMinRestInMs * time.Millisecond
var watchBufferMaxDuration = watchBufferMaxTimeInMs * time.Millisecond

// TODO(nick): maybe this should be called 'BuildEngine' or something?
// Upper seems like a poor and undescriptive name.
type Upper struct {
	store *store.Store
}

type FsWatcherMaker func(paths []string, ignore watch.PathMatcher, l logger.Logger) (watch.Notify, error)
type ServiceWatcherMaker func(context.Context, *store.Store) error
type PodWatcherMaker func(context.Context, *store.Store) error
type timerMaker func(d time.Duration) <-chan time.Time

func ProvideFsWatcherMaker() FsWatcherMaker {
	return func(paths []string, ignore watch.PathMatcher, l logger.Logger) (watch.Notify, error) {
		return watch.NewWatcher(paths, ignore, l)
	}
}

func ProvideTimerMaker() timerMaker {
	return func(t time.Duration) <-chan time.Time {
		return time.After(t)
	}
}

func NewUpper(ctx context.Context, st *store.Store, subs []store.Subscriber) Upper {
	// There's not really a good reason to add all the subscribers
	// in NewUpper(), but it's as good a place as any.
	for _, sub := range subs {
		st.AddSubscriber(ctx, sub)
	}

	return Upper{
		store: st,
	}
}

func (u Upper) Dispatch(action store.Action) {
	u.store.Dispatch(action)
}

func (u Upper) Start(
	ctx context.Context,
	args []string,
	b model.TiltBuild,
	watch bool,
	fileName string,
	hudEnabled bool,
	analyticsUserOpt analytics.Opt,
	token token.Token,
	cloudAddress string,
) error {

	span, ctx := opentracing.StartSpanFromContext(ctx, "Start")
	defer span.Finish()

	startTime := time.Now()

	absTfPath, err := filepath.Abs(fileName)
	if err != nil {
		return err
	}

	var manifestNames []model.ManifestName
	for _, arg := range args {
		manifestNames = append(manifestNames, model.ManifestName(arg))
	}

	configFiles := []string{absTfPath}

	return u.Init(ctx, InitAction{
		WatchFiles:       watch,
		TiltfilePath:     absTfPath,
		ConfigFiles:      configFiles,
		InitManifests:    manifestNames,
		TiltBuild:        b,
		StartTime:        startTime,
		AnalyticsUserOpt: analyticsUserOpt,
		Token:            token,
		CloudAddress:     cloudAddress,
		HUDEnabled:       hudEnabled,
	})
}

func (u Upper) Init(ctx context.Context, action InitAction) error {
	u.store.Dispatch(action)
	return u.store.Loop(ctx)
}

func upperReducerFn(ctx context.Context, state *store.EngineState, action store.Action) {
	// Allow exitAction and dumpEngineStateAction even if there's a fatal error
	if exitAction, isExitAction := action.(hud.ExitAction); isExitAction {
		handleExitAction(state, exitAction)
		return
	}
	if _, isDumpEngineStateAction := action.(hud.DumpEngineStateAction); isDumpEngineStateAction {
		handleDumpEngineStateAction(ctx, state)
		return
	}

	if state.FatalError != nil {
		return
	}

	logAction, isLogAction := action.(store.LogAction)
	if isLogAction {
		handleLogAction(state, logAction)
	}

	var err error
	switch action := action.(type) {
	case InitAction:
		err = handleInitAction(ctx, state, action)
	case store.ErrorAction:
		err = action.Error
	case hud.ExitAction:
		handleExitAction(state, action)
	case targetFilesChangedAction:
		handleFSEvent(ctx, state, action)
	case k8swatch.PodChangeAction:
		handlePodChangeAction(ctx, state, action)
	case store.PodResetRestartsAction:
		handlePodResetRestartsAction(state, action)
	case k8swatch.ServiceChangeAction:
		handleServiceEvent(ctx, state, action)
	case store.K8sEventAction:
		handleK8sEvent(ctx, state, action)
	case runtimelog.PodLogAction:
		handlePodLogAction(state, action)
	case BuildLogAction:
		handleBuildLogAction(state, action)
	case BuildCompleteAction:
		err = handleBuildCompleted(ctx, state, action)
	case BuildStartedAction:
		handleBuildStarted(ctx, state, action)
	case configs.ConfigsReloadStartedAction:
		handleConfigsReloadStarted(ctx, state, action)
	case configs.ConfigsReloadedAction:
		handleConfigsReloaded(ctx, state, action)
	case DockerComposeEventAction:
		handleDockerComposeEvent(ctx, state, action)
	case runtimelog.DockerComposeLogAction:
		handleDockerComposeLogAction(state, action)
	case server.AppendToTriggerQueueAction:
		appendToTriggerQueue(state, action.Name)
	case hud.StartProfilingAction:
		handleStartProfilingAction(state)
	case hud.StopProfilingAction:
		handleStopProfilingAction(state)
	case hud.SetLogTimestampsAction:
		handleLogTimestampsAction(state, action)
	case configs.TiltfileLogAction:
		handleTiltfileLogAction(ctx, state, action)
	case hud.DumpEngineStateAction:
		handleDumpEngineStateAction(ctx, state)
	case LatestVersionAction:
		handleLatestVersionAction(state, action)
	case store.AnalyticsUserOptAction:
		handleAnalyticsUserOptAction(state, action)
	case store.AnalyticsNudgeSurfacedAction:
		handleAnalyticsNudgeSurfacedAction(ctx, state)
	case store.TiltCloudUserLookedUpAction:
		handleTiltCloudUserLookedUpAction(state, action)
	case store.UserStartedTiltCloudRegistrationAction:
		handleUserStartedTiltCloudRegistrationAction(state)
	case store.LogEvent:
		// handled as a LogAction, do nothing

	default:
		err = fmt.Errorf("unrecognized action: %T", action)
	}

	if err != nil {
		state.FatalError = err
	}
}

var UpperReducer = store.Reducer(upperReducerFn)

func handleBuildStarted(ctx context.Context, state *store.EngineState, action BuildStartedAction) {
	mn := action.ManifestName
	ms, ok := state.ManifestState(mn)
	if !ok {
		return
	}

	bs := model.BuildRecord{
		Edits:     append([]string{}, action.FilesChanged...),
		StartTime: action.StartTime,
		Reason:    action.Reason,
	}
	ms.ConfigFilesThatCausedChange = []string{}
	ms.CurrentBuild = bs

	if ms.IsK8s() {
		for _, pod := range ms.K8sRuntimeState().Pods {
			pod.CurrentLog = model.Log{}
			pod.UpdateStartTime = action.StartTime
		}
	} else if ms.IsDC() {
		ms.RuntimeState = ms.DCRuntimeState().WithCurrentLog(model.Log{})
	}

	// Keep the crash log around until we have a rebuild
	// triggered by a explicit change (i.e., not a crash rebuild)
	if !action.Reason.IsCrashOnly() {
		ms.CrashLog = model.Log{}
	}

	state.CurrentlyBuilding = mn
	removeFromTriggerQueue(state, mn)
}

func handleBuildCompleted(ctx context.Context, engineState *store.EngineState, cb BuildCompleteAction) error {
	defer func() {
		engineState.CurrentlyBuilding = ""

		if engineState.InitialBuildsCompleted() {
			logger.Get(ctx).Debugf("[timing.py] finished initial build") // hook for timing.py
		}
	}()

	engineState.CompletedBuildCount++
	engineState.BuildControllerActionCount++
	err := cb.Error

	mt, ok := engineState.ManifestTargets[engineState.CurrentlyBuilding]
	if !ok {
		return nil
	}

	ms := mt.State
	bs := ms.CurrentBuild
	bs.Error = err
	bs.FinishTime = time.Now()
	bs.BuildTypes = cb.Result.BuildTypes()
	ms.AddCompletedBuild(bs)

	ms.CurrentBuild = model.BuildRecord{}
	ms.NeedsRebuildFromCrash = false

	for id, result := range cb.Result {
		ms.MutableBuildStatus(id).LastResult = result
	}

	if err != nil {
		if isFatalError(err) {
			return err
		} else if engineState.WatchFiles {
			l := logger.Get(ctx)
			p := logger.Red(l).Sprintf("Build Failed:")
			l.Infof("%s %v", p, err)
		} else {
			return errors.Wrap(err, "Build Failed")
		}
	} else {
		// Remove pending file changes that were consumed by this build.
		for _, status := range ms.BuildStatuses {
			for file, modTime := range status.PendingFileChanges {
				if modTime.Before(bs.StartTime) {
					delete(status.PendingFileChanges, file)
				}
			}
		}

		if !ms.PendingManifestChange.IsZero() &&
			ms.PendingManifestChange.Before(bs.StartTime) {
			ms.PendingManifestChange = time.Time{}
		}

		ms.LastSuccessfulDeployTime = time.Now()

		for id, result := range cb.Result {
			ms.MutableBuildStatus(id).LastSuccessfulResult = result
		}

		for _, pod := range ms.K8sRuntimeState().Pods {
			// Reset the baseline, so that we don't show restarts
			// from before any live-updates
			pod.BaselineRestarts = pod.AllContainerRestarts()
		}
	}

	// Track the container ids that have been live-updated whether the
	// build succeeds or fails.
	liveUpdateContainerIDs := cb.Result.LiveUpdatedContainerIDs()
	if len(liveUpdateContainerIDs) == 0 {
		// Assume this was an image build, and reset all the container ids
		ms.LiveUpdatedContainerIDs = container.NewIDSet()
	} else {
		for _, cID := range liveUpdateContainerIDs {
			ms.LiveUpdatedContainerIDs[cID] = true
		}

		bestPod := ms.MostRecentPod()
		if bestPod.StartedAt.After(bs.StartTime) ||
			bestPod.UpdateStartTime.Equal(bs.StartTime) {
			checkForContainerCrash(ctx, engineState, mt)
		}
	}

	manifest := mt.Manifest
	deployedUIDSet := cb.Result.DeployedUIDSet()
	if manifest.IsK8s() && len(deployedUIDSet) > 0 {
		state := ms.GetOrCreateK8sRuntimeState()
		state.DeployedUIDSet = deployedUIDSet
		ms.RuntimeState = state
	}

	if mt.Manifest.IsDC() {
		state, _ := ms.RuntimeState.(dockercompose.State)

		result := cb.Result[mt.Manifest.DockerComposeTarget().ID()]
		dcResult, _ := result.(store.DockerComposeBuildResult)
		cid := dcResult.DockerComposeContainerID
		if cid != "" {
			state = state.WithContainerID(cid)
		}

		// If we have a container ID and no status yet, set status to Up
		// (this is an expected case when we run docker-compose up while the service
		// is already running, and we won't get an event to tell us so).
		// If the container is crashing we will get an event subsequently.
		isFirstBuild := cid != "" && state.Status == ""
		if isFirstBuild {
			state = state.WithStatus(dockercompose.StatusUp)
		}

		ms.RuntimeState = state
	}

	if mt.Manifest.IsLocal() {
		ms.RuntimeState = store.LocalRuntimeState{HasSucceededAtLeastOnce: err == nil}
	}

	if engineState.WatchFiles {
		logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py
	}

	return nil
}

func appendToTriggerQueue(state *store.EngineState, mn model.ManifestName) {
	_, ok := state.ManifestState(mn)
	if !ok {
		return
	}

	for _, triggerName := range state.TriggerQueue {
		if mn == triggerName {
			return
		}
	}
	state.TriggerQueue = append(state.TriggerQueue, mn)
}

func removeFromTriggerQueue(state *store.EngineState, mn model.ManifestName) {
	for i, triggerName := range state.TriggerQueue {
		if triggerName == mn {
			state.TriggerQueue = append(state.TriggerQueue[:i], state.TriggerQueue[i+1:]...)
			break
		}
	}
}

func handleStopProfilingAction(state *store.EngineState) {
	state.IsProfiling = false
}

func handleStartProfilingAction(state *store.EngineState) {
	state.IsProfiling = true
}

func handleLogTimestampsAction(state *store.EngineState, action hud.SetLogTimestampsAction) {
	state.LogTimestamps = action.Value
}

func handleFSEvent(
	ctx context.Context,
	state *store.EngineState,
	event targetFilesChangedAction) {

	if event.targetID.Type == model.TargetTypeConfigs {
		for _, f := range event.files {
			state.PendingConfigFileChanges[f] = event.time
		}
		return
	}

	mns := state.ManifestNamesForTargetID(event.targetID)
	for _, mn := range mns {
		ms, ok := state.ManifestState(mn)
		if !ok {
			return
		}

		status := ms.MutableBuildStatus(event.targetID)
		for _, f := range event.files {
			status.PendingFileChanges[f] = event.time
		}
	}
}

func handleConfigsReloadStarted(
	ctx context.Context,
	state *store.EngineState,
	event configs.ConfigsReloadStartedAction,
) {
	filesChanged := []string{}
	for f, _ := range event.FilesChanged {
		filesChanged = append(filesChanged, f)
	}
	status := model.BuildRecord{
		StartTime: event.StartTime,
		Reason:    model.BuildReasonFlagConfig,
		Edits:     filesChanged,
	}

	state.TiltfileState.CurrentBuild = status
}

func handleConfigsReloaded(
	ctx context.Context,
	state *store.EngineState,
	event configs.ConfigsReloadedAction,
) {
	manifests := event.Manifests

	b := state.TiltfileState.CurrentBuild

	// Track the new secrets and go back to scrub them.
	newSecrets := model.SecretSet{}
	for k, v := range event.Secrets {
		_, exists := state.Secrets[k]
		if !exists {
			newSecrets[k] = v
		}
	}

	// Add all secrets, even if we failed.
	state.Secrets.AddAll(event.Secrets)

	// Retroactively scrub secrets
	b.Log.ScrubSecretsStartingAt(newSecrets, 0)
	state.Log.ScrubSecretsStartingAt(newSecrets, event.GlobalLogLineCountAtExecStart)

	// if the ConfigsReloadedAction came from a unit test, there might not be a current build
	if !b.Empty() {
		b.FinishTime = event.FinishTime
		b.Error = event.Err
		b.Warnings = event.Warnings

		state.TiltfileState.AddCompletedBuild(b)
	}
	state.TiltfileState.CurrentBuild = model.BuildRecord{}
	if event.Err != nil {
		// When the Tiltfile had an error, we want to differentiate between two cases:
		//
		// 1) You're running `tilt up` for the first time, and a local() command
		// exited with status code 1.  Partial results (like enabling features)
		// would be helpful.
		//
		// 2) You're running 'tilt up' in the happy state. You edit the Tiltfile,
		// and introduce a syntax error.  You don't want partial results to wipe out
		// your "good" state.

		// Watch any new config files in the partial state.
		state.ConfigFiles = sliceutils.AppendWithoutDupes(state.ConfigFiles, event.ConfigFiles...)

		// Enable any new features in the partial state.
		if len(state.Features) == 0 {
			state.Features = event.Features
		} else {
			for feature, val := range event.Features {
				if val {
					state.Features[feature] = val
				}
			}
		}
		return
	}

	state.DockerPruneSettings = event.DockerPruneSettings

	newDefOrder := make([]model.ManifestName, len(manifests))
	for i, m := range manifests {
		mt, ok := state.ManifestTargets[m.ManifestName()]
		if !ok {
			mt = store.NewManifestTarget(m)
		}

		newDefOrder[i] = m.ManifestName()

		configFilesThatChanged := state.TiltfileState.LastBuild().Edits
		old := mt.Manifest
		mt.Manifest = m

		if model.ChangesInvalidateBuild(old, m) {
			// Manifest has changed such that the current build is invalid;
			// ensure we do an image build so that we apply the changes
			ms := mt.State
			ms.BuildStatuses = make(map[model.TargetID]*store.BuildStatus)
			ms.PendingManifestChange = time.Now()
			ms.ConfigFilesThatCausedChange = configFilesThatChanged
		}
		state.UpsertManifestTarget(mt)
	}
	// TODO(dmiller) handle deleting manifests
	// TODO(maia): update ConfigsManifest with new ConfigFiles/update watches
	state.ManifestDefinitionOrder = newDefOrder
	state.ConfigFiles = event.ConfigFiles
	state.TiltIgnoreContents = event.TiltIgnoreContents

	state.Features = event.Features
	state.TeamName = event.TeamName

	state.VersionSettings = event.VersionSettings

	// Remove pending file changes that were consumed by this build.
	for file, modTime := range state.PendingConfigFileChanges {
		if modTime.Before(state.TiltfileState.LastBuild().StartTime) {
			delete(state.PendingConfigFileChanges, file)
		}
	}
}

func handleBuildLogAction(state *store.EngineState, action BuildLogAction) {
	manifestName := action.Source()
	ms, ok := state.ManifestState(manifestName)
	if !ok || state.CurrentlyBuilding != manifestName {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	ms.CurrentBuild.Log = model.AppendLog(ms.CurrentBuild.Log, action, state.LogTimestamps, "", state.Secrets)
}

func handleLogAction(state *store.EngineState, action store.LogAction) {
	manifestName := action.Source()
	alreadyHasSourcePrefix := false
	if _, isDCLog := action.(runtimelog.DockerComposeLogAction); isDCLog {
		// DockerCompose logs are prefixed by the docker-compose engine
		alreadyHasSourcePrefix = true
	}

	var allLogPrefix string
	if manifestName != "" && !alreadyHasSourcePrefix {
		allLogPrefix = sourcePrefix(manifestName)
	}

	state.Log = model.AppendLog(state.Log, action, state.LogTimestamps, allLogPrefix, state.Secrets)

	if manifestName == "" {
		return
	}

	ms, ok := state.ManifestState(manifestName)
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}
	ms.CombinedLog = model.AppendLog(ms.CombinedLog, action, state.LogTimestamps, "", state.Secrets)
}

func sourcePrefix(n model.ManifestName) string {
	max := 12
	spaces := ""
	if len(n) > max {
		n = n[:max-1] + "…"
	} else {
		spaces = strings.Repeat(" ", max-len(n))
	}
	return fmt.Sprintf("%s%s┊ ", n, spaces)
}

func handleServiceEvent(ctx context.Context, state *store.EngineState, action k8swatch.ServiceChangeAction) {
	service := action.Service
	ms, ok := state.ManifestState(action.ManifestName)
	if !ok {
		return
	}

	runtime := ms.GetOrCreateK8sRuntimeState()
	runtime.LBs[k8s.ServiceName(service.Name)] = action.URL
}

func handleK8sEvent(ctx context.Context, state *store.EngineState, action store.K8sEventAction) {
	evt := action.Event

	if evt.Type != v1.EventTypeNormal {
		handleLogAction(state, action.ToLogAction(action.ManifestName))
	}
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

func handleLatestVersionAction(state *store.EngineState, action LatestVersionAction) {
	state.LatestTiltBuild = action.Build
}

func handleInitAction(ctx context.Context, engineState *store.EngineState, action InitAction) error {
	engineState.TiltBuildInfo = action.TiltBuild
	engineState.TiltStartTime = action.StartTime
	engineState.TiltfilePath = action.TiltfilePath
	engineState.ConfigFiles = action.ConfigFiles
	engineState.InitManifests = action.InitManifests
	engineState.AnalyticsUserOpt = action.AnalyticsUserOpt
	engineState.WatchFiles = action.WatchFiles
	engineState.CloudAddress = action.CloudAddress
	engineState.Token = action.Token
	engineState.HUDEnabled = action.HUDEnabled

	// NOTE(dmiller): this kicks off a Tiltfile build
	engineState.PendingConfigFileChanges[action.TiltfilePath] = time.Now()

	return nil
}

func handleExitAction(state *store.EngineState, action hud.ExitAction) {
	if action.Err != nil {
		state.FatalError = action.Err
	} else {
		state.UserExited = true
	}
}

func handleDockerComposeEvent(ctx context.Context, engineState *store.EngineState, action DockerComposeEventAction) {
	evt := action.Event
	mn := evt.Service
	ms, ok := engineState.ManifestState(model.ManifestName(mn))
	if !ok {
		// No corresponding manifest, nothing to do
		return
	}

	if evt.Type != dockercompose.TypeContainer {
		// We currently only support Container events.
		return
	}

	state, _ := ms.RuntimeState.(dockercompose.State)

	state = state.WithContainerID(container.ID(evt.ID))

	// For now, just guess at state.
	status := evt.GuessStatus()
	if status != "" {
		state = state.WithStatus(status)
	}

	if evt.IsStartupEvent() {
		state = state.WithStartTime(time.Now())
		state = state.WithStopping(false)
		// NB: this will differ from StartTime once we support DC health checks
		state = state.WithLastReadyTime(time.Now())
	}

	if evt.IsStopEvent() {
		state = state.WithStopping(true)
	}

	if evt.Action == dockercompose.ActionDie && !state.IsStopping {
		state = state.WithStatus(dockercompose.StatusCrash)
	}

	ms.RuntimeState = state
}

func handleDockerComposeLogAction(state *store.EngineState, action runtimelog.DockerComposeLogAction) {
	manifestName := action.Source()
	ms, ok := state.ManifestState(manifestName)
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	dcState, _ := ms.RuntimeState.(dockercompose.State)
	ms.RuntimeState = dcState.WithCurrentLog(model.AppendLog(dcState.CurrentLog, action, state.LogTimestamps, "", state.Secrets))
}

func handleTiltfileLogAction(ctx context.Context, state *store.EngineState, action configs.TiltfileLogAction) {
	state.TiltfileState.CurrentBuild.Log = model.AppendLog(state.TiltfileState.CurrentBuild.Log, action, state.LogTimestamps, "", state.Secrets)
	state.TiltfileState.CombinedLog = model.AppendLog(state.TiltfileState.CombinedLog, action, state.LogTimestamps, "", state.Secrets)
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
		tiltanalytics.Get(ctx).IncrIfUnopted("analytics.nudge.surfaced")
		state.AnalyticsNudgeSurfaced = true
	}
}

func handleTiltCloudUserLookedUpAction(state *store.EngineState, action store.TiltCloudUserLookedUpAction) {
	if action.IsPostRegistrationLookup {
		state.WaitingForTiltCloudUsernamePostRegistration = false
	}
	if !action.Found {
		state.TokenKnownUnregistered = true
		state.TiltCloudUsername = ""
	} else {
		state.TokenKnownUnregistered = false
		state.TiltCloudUsername = action.Username
	}
}

func handleUserStartedTiltCloudRegistrationAction(state *store.EngineState) {
	state.WaitingForTiltCloudUsernamePostRegistration = true
}
