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
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/watch"
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

// When we kick off a build because some files changed, only print the first `maxChangedFilesToPrint`
const maxChangedFilesToPrint = 5

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
	useActionWriter bool,
	sailMode model.SailMode,
	analyticsOpt analytics.Opt) error {

	span, ctx := opentracing.StartSpanFromContext(ctx, "Start")
	defer span.Finish()

	startTime := time.Now()

	absTfPath, err := filepath.Abs(fileName)
	if err != nil {
		return err
	}

	var manifestNames []model.ManifestName
	matching := map[string]bool{}
	for _, arg := range args {
		manifestNames = append(manifestNames, model.ManifestName(arg))
		matching[arg] = true
	}

	configFiles := []string{absTfPath}

	return u.Init(ctx, InitAction{
		WatchFiles:      watch,
		TiltfilePath:    absTfPath,
		ConfigFiles:     configFiles,
		InitManifests:   manifestNames,
		TiltBuild:       b,
		StartTime:       startTime,
		FinishTime:      time.Now(),
		ExecuteTiltfile: false,
		EnableSail:      sailMode.IsEnabled(),
		AnalyticsOpt:    analyticsOpt,
	})
}

func (u Upper) Init(ctx context.Context, action InitAction) error {
	u.store.Dispatch(action)
	return u.store.Loop(ctx)
}

var UpperReducer = store.Reducer(func(ctx context.Context, state *store.EngineState, action store.Action) {
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
	case PodChangeAction:
		handlePodChangeAction(ctx, state, action.Pod)
	case ServiceChangeAction:
		handleServiceEvent(ctx, state, action)
	case store.K8sEventAction:
		handleK8sEvent(ctx, state, action)
	case PodLogAction:
		handlePodLogAction(state, action)
	case BuildLogAction:
		handleBuildLogAction(state, action)
	case BuildCompleteAction:
		err = handleBuildCompleted(ctx, state, action)
	case BuildStartedAction:
		handleBuildStarted(ctx, state, action)
	case DeployIDAction:
		handleDeployIDAction(ctx, state, action)
	case ConfigsReloadStartedAction:
		handleConfigsReloadStarted(ctx, state, action)
	case ConfigsReloadedAction:
		handleConfigsReloaded(ctx, state, action)
	case DockerComposeEventAction:
		handleDockerComposeEvent(ctx, state, action)
	case DockerComposeLogAction:
		handleDockerComposeLogAction(state, action)
	case server.AppendToTriggerQueueAction:
		appendToTriggerQueue(state, action.Name)
	case hud.StartProfilingAction:
		handleStartProfilingAction(state)
	case hud.StopProfilingAction:
		handleStopProfilingAction(state)
	case hud.SetLogTimestampsAction:
		handleLogTimestampsAction(state, action)
	case client.SailRoomConnectedAction:
		handleSailRoomConnectedAction(ctx, state, action)
	case TiltfileLogAction:
		handleTiltfileLogAction(ctx, state, action)
	case hud.DumpEngineStateAction:
		handleDumpEngineStateAction(ctx, state)
	case LatestVersionAction:
		handleLatestVersionAction(state, action)
	case store.AnalyticsOptAction:
		handleAnalyticsOptAction(state, action)
	case store.AnalyticsNudgeSurfacedAction:
		handleAnalyticsNudgeSurfacedAction(ctx, state)
	case store.LogEvent:
		// handled as a LogAction, do nothing

	default:
		err = fmt.Errorf("unrecognized action: %T", action)
	}

	if err != nil {
		state.PermanentError = err
	}
})

func handleBuildStarted(ctx context.Context, state *store.EngineState, action BuildStartedAction) {
	mn := action.ManifestName
	ms, ok := state.ManifestState(mn)
	if !ok {
		return
	}

	edits := []string{}
	edits = append(edits, action.FilesChanged...)

	bs := model.BuildRecord{
		Edits:     append(edits, ms.ConfigFilesThatCausedChange...),
		StartTime: action.StartTime,
		Reason:    action.Reason,
	}
	ms.ConfigFilesThatCausedChange = []string{}
	ms.CurrentBuild = bs

	for _, pod := range ms.PodSet.Pods {
		pod.CurrentLog = model.Log{}
		pod.UpdateStartTime = action.StartTime
	}

	if dcState, ok := ms.ResourceState.(dockercompose.State); ok {
		ms.ResourceState = dcState.WithCurrentLog(model.Log{})
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
	}()

	engineState.CompletedBuildCount++
	engineState.BuildControllerActionCount++

	defer func() {
		if engineState.CompletedBuildCount == engineState.InitialBuildsQueued {
			logger.Get(ctx).Debugf("[timing.py] finished initial build") // hook for timing.py
		}
	}()

	err := cb.Error

	mt, ok := engineState.ManifestTargets[engineState.CurrentlyBuilding]
	if !ok {
		return nil
	}

	ms := mt.State
	bs := ms.CurrentBuild
	bs.Error = err
	bs.FinishTime = time.Now()
	ms.AddCompletedBuild(bs)

	ms.CurrentBuild = model.BuildRecord{}
	ms.NeedsRebuildFromCrash = false

	if err != nil {
		if isPermanentError(err) {
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

		for _, pod := range ms.PodSet.Pods {
			// # of pod restarts from old code (shouldn't be reflected in HUD)
			pod.OldRestarts = pod.AllContainerRestarts()
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

	if mt.Manifest.IsDC() {
		state, _ := ms.ResourceState.(dockercompose.State)

		dcResult := cb.Result[mt.Manifest.DockerComposeTarget().ID()]
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

		ms.ResourceState = state
	}

	if engineState.WatchFiles {
		logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py
	}

	return nil
}

func handleDeployIDAction(ctx context.Context, state *store.EngineState, action DeployIDAction) {
	mns := state.ManifestNamesForTargetID(action.TargetID)
	for _, mn := range mns {
		ms, ok := state.ManifestState(mn)
		if !ok {
			continue
		}

		ms.DeployID = action.DeployID
	}
}

func appendToTriggerQueue(state *store.EngineState, mn model.ManifestName) {
	ms, ok := state.ManifestState(mn)
	if !ok {
		return
	}

	ok, _ = ms.HasPendingChanges()
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

func handleSailRoomConnectedAction(ctx context.Context, state *store.EngineState, action client.SailRoomConnectedAction) {
	if action.Err != nil {
		logger.Get(ctx).Infof("Error connecting Sail room: %v\n", action.Err)
		return
	}
	state.SailURL = action.ViewURL
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
	event ConfigsReloadStartedAction,
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

	state.CurrentTiltfileBuild = status
}

func handleConfigsReloaded(
	ctx context.Context,
	state *store.EngineState,
	event ConfigsReloadedAction,
) {
	state.FirstTiltfileBuildCompleted = true
	manifests := event.Manifests
	if state.InitialBuildsQueued == 0 {
		state.InitialBuildsQueued = len(manifests)
	}

	status := state.CurrentTiltfileBuild
	status.FinishTime = event.FinishTime
	status.Error = event.Err
	status.Warnings = event.Warnings

	state.TeamName = event.TeamName

	state.LastTiltfileBuild = status
	state.CurrentTiltfileBuild = model.BuildRecord{}
	if event.Err != nil {
		// There was an error, so don't update status with the new, nonexistent state

		// EXCEPT for the config file list, because we want to watch new config files even when the tiltfile is broken
		// append any new config files found in the reload action
		state.ConfigFiles = sliceutils.AppendWithoutDupes(state.ConfigFiles, event.ConfigFiles...)

		return
	}

	newDefOrder := make([]model.ManifestName, len(manifests))
	for i, m := range manifests {
		mt, ok := state.ManifestTargets[m.ManifestName()]
		if !ok {
			mt = store.NewManifestTarget(m)
		}

		newDefOrder[i] = m.ManifestName()

		configFilesThatChanged := state.LastTiltfileBuild.Edits
		old := mt.Manifest
		mt.Manifest = m
		if model.ChangesInvalidateBuild(old, m) {
			// Manifest has changed such that the current build is invalid;
			// ensure we do an image build so that we apply the changes
			state := mt.State
			state.BuildStatuses = make(map[model.TargetID]*store.BuildStatus)
			state.PendingManifestChange = time.Now()
			state.ConfigFilesThatCausedChange = configFilesThatChanged
		}
		state.UpsertManifestTarget(mt)
	}
	// TODO(dmiller) handle deleting manifests
	// TODO(maia): update ConfigsManifest with new ConfigFiles/update watches
	state.ManifestDefinitionOrder = newDefOrder
	state.ConfigFiles = event.ConfigFiles
	state.TiltIgnoreContents = event.TiltIgnoreContents

	state.Features = event.Features

	// Remove pending file changes that were consumed by this build.
	for file, modTime := range state.PendingConfigFileChanges {
		if modTime.Before(status.StartTime) {
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

	ms.CurrentBuild.Log = model.AppendLog(ms.CurrentBuild.Log, action, state.LogTimestamps, "")
}

func handleLogAction(state *store.EngineState, action store.LogAction) {
	manifestName := action.Source()
	alreadyHasSourcePrefix := false
	if _, isDCLog := action.(DockerComposeLogAction); isDCLog {
		// DockerCompose logs are prefixed by the docker-compose engine
		alreadyHasSourcePrefix = true
	}

	var allLogPrefix string
	if manifestName != "" && !alreadyHasSourcePrefix {
		allLogPrefix = sourcePrefix(manifestName)
	}

	state.Log = model.AppendLog(state.Log, action, state.LogTimestamps, allLogPrefix)

	if manifestName == "" {
		return
	}

	ms, ok := state.ManifestState(manifestName)
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}
	ms.CombinedLog = model.AppendLog(ms.CombinedLog, action, state.LogTimestamps, "")
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

func handleServiceEvent(ctx context.Context, state *store.EngineState, action ServiceChangeAction) {
	service := action.Service
	manifestName := model.ManifestName(service.ObjectMeta.Labels[k8s.ManifestNameLabel])
	if manifestName == "" || manifestName == model.UnresourcedYAMLManifestName {
		return
	}

	ms, ok := state.ManifestState(manifestName)
	if !ok {
		return
	}

	ms.LBs[k8s.ServiceName(service.Name)] = action.URL
}

func handleK8sEvent(ctx context.Context, state *store.EngineState, action store.K8sEventAction) {
	evt := action.Event

	if evt.Type != v1.EventTypeNormal {
		handleLogAction(state, action.ToLogAction(action.ManifestName))

		ms, ok := state.ManifestState(action.ManifestName)
		if !ok {
			return
		}
		ms.K8sWarnEvents = append(ms.K8sWarnEvents, k8s.NewEventWithEntity(evt, action.InvolvedObject))
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
	watchFiles := action.WatchFiles
	engineState.TiltBuildInfo = action.TiltBuild
	engineState.TiltStartTime = action.StartTime
	engineState.TiltfilePath = action.TiltfilePath
	engineState.ConfigFiles = action.ConfigFiles
	engineState.InitManifests = action.InitManifests
	engineState.SailEnabled = action.EnableSail

	engineState.AnalyticsOpt = action.AnalyticsOpt

	if action.ExecuteTiltfile {
		status := model.BuildRecord{
			StartTime:  action.StartTime,
			FinishTime: action.FinishTime,
			Error:      action.Err,
			Warnings:   action.Warnings,
			Reason:     model.BuildReasonFlagInit,
		}
		engineState.LastTiltfileBuild = status

		manifests := action.Manifests
		for _, m := range manifests {
			engineState.UpsertManifestTarget(store.NewManifestTarget(m))
		}

		engineState.InitialBuildsQueued = len(manifests)
	} else {
		// NOTE(dmiller): this kicks off a Tiltfile build
		engineState.PendingConfigFileChanges[action.TiltfilePath] = time.Now()
	}

	engineState.WatchFiles = watchFiles
	return nil
}

func handleExitAction(state *store.EngineState, action hud.ExitAction) {
	if action.Err != nil {
		state.PermanentError = action.Err
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

	state, _ := ms.ResourceState.(dockercompose.State)

	state = state.WithContainerID(container.ID(evt.ID))

	// For now, just guess at state.
	status := evt.GuessStatus()
	if status != "" {
		state = state.WithStatus(status)
	}

	if evt.IsStartupEvent() {
		state = state.WithStartTime(time.Now())
		state = state.WithStopping(false)
	}

	if evt.IsStopEvent() {
		state = state.WithStopping(true)
	}

	if evt.Action == dockercompose.ActionDie && !state.IsStopping {
		state = state.WithStatus(dockercompose.StatusCrash)
	}

	ms.ResourceState = state
}

func handleDockerComposeLogAction(state *store.EngineState, action DockerComposeLogAction) {
	manifestName := action.Source()
	ms, ok := state.ManifestState(manifestName)
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	dcState, _ := ms.ResourceState.(dockercompose.State)
	ms.ResourceState = dcState.WithCurrentLog(model.AppendLog(dcState.CurrentLog, action, state.LogTimestamps, ""))
}

func handleTiltfileLogAction(ctx context.Context, state *store.EngineState, action TiltfileLogAction) {
	state.CurrentTiltfileBuild.Log = model.AppendLog(state.CurrentTiltfileBuild.Log, action, state.LogTimestamps, "")
	state.TiltfileCombinedLog = model.AppendLog(state.TiltfileCombinedLog, action, state.LogTimestamps, "")
}

func handleAnalyticsOptAction(state *store.EngineState, action store.AnalyticsOptAction) {
	state.AnalyticsOpt = action.Opt
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
