package engine

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/windmilleng/wmclient/pkg/analytics"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8swatch "k8s.io/apimachinery/pkg/watch"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/sail/client"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
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

type FsWatcherMaker func() (watch.Notify, error)
type ServiceWatcherMaker func(context.Context, *store.Store) error
type PodWatcherMaker func(context.Context, *store.Store) error
type timerMaker func(d time.Duration) <-chan time.Time

func ProvideFsWatcherMaker() FsWatcherMaker {
	return func() (watch.Notify, error) {
		return watch.NewWatcher()
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
	triggerMode model.TriggerMode,
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
		TriggerMode:     triggerMode,
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
	case store.K8SEventAction:
		handleK8SEvent(ctx, state, action)
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
	case store.LogAction:
		handleLogAction(state, action)
	case ConfigsReloadStartedAction:
		handleConfigsReloadStarted(ctx, state, action)
	case ConfigsReloadedAction:
		handleConfigsReloaded(ctx, state, action)
	case DockerComposeEventAction:
		handleDockerComposeEvent(ctx, state, action)
	case DockerComposeLogAction:
		handleDockerComposeLogAction(state, action)
	case view.AppendToTriggerQueueAction:
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
	case UIDUpdateAction:
		handleUIDUpdateAction(state, action)
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
	ms.ExpectedContainerID = ""

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
			pod.OldRestarts = pod.ContainerRestarts
		}
	}

	if mt.Manifest.IsDC() {
		state, _ := ms.ResourceState.(dockercompose.State)

		cid := cb.Result.OneAndOnlyContainerID()
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

		cID := cb.Result.OneAndOnlyContainerID()
		if cID != "" {
			ms.ExpectedContainerID = cID

			bestPod := ms.MostRecentPod()
			if bestPod.StartedAt.After(bs.StartTime) ||
				bestPod.UpdateStartTime.Equal(bs.StartTime) {
				checkForPodCrash(ctx, engineState, ms, bestPod)
			}
		}
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
	if state.TriggerMode != model.TriggerManual {
		return
	}

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
		if !m.Equal(mt.Manifest) {
			mt.Manifest = m

			// Manifest has changed, ensure we do an image build so that we apply the changes
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

	// Remove pending file changes that were consumed by this build.
	for file, modTime := range state.PendingConfigFileChanges {
		if modTime.Before(status.StartTime) {
			delete(state.PendingConfigFileChanges, file)
		}
	}
}

// Get a pointer to a mutable manifest state,
// ensuring that some Pod exists on the state.
//
// Intended as a helper for pod-mutating events.
func ensureManifestTargetWithPod(state *store.EngineState, pod *v1.Pod) (*store.ManifestTarget, *store.Pod) {
	manifestName := model.ManifestName(pod.ObjectMeta.Labels[k8s.ManifestNameLabel])
	if manifestName == "" {
		// if there's no ManifestNameLabel, then maybe it matches some manifest's ExtraPodSelectors
		for _, m := range state.Manifests() {
			if m.IsK8s() {
				for _, lps := range m.K8sTarget().ExtraPodSelectors {
					if lps.Matches(labels.Set(pod.ObjectMeta.GetLabels())) {
						manifestName = m.Name
						break
					}
				}
			}
		}
	}

	mt, ok := state.ManifestTargets[manifestName]
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return nil, nil
	}

	ms := mt.State

	deployID := ms.DeployID
	if podDeployID, ok := pod.ObjectMeta.Labels[k8s.TiltDeployIDLabel]; ok {
		if pdID, err := strconv.Atoi(podDeployID); err != nil || pdID != int(deployID) {
			return nil, nil
		}
	}

	podID := k8s.PodIDFromPod(pod)
	startedAt := pod.CreationTimestamp.Time
	status := podStatusToString(*pod)
	ns := k8s.NamespaceFromPod(pod)
	hasSynclet := sidecar.PodSpecContainsSynclet(pod.Spec)

	// CASE 1: We don't have a set of pods for this DeployID yet
	if ms.PodSet.DeployID == 0 || ms.PodSet.DeployID != deployID {
		ms.PodSet = store.PodSet{
			DeployID: deployID,
			Pods:     make(map[k8s.PodID]*store.Pod),
		}
		ms.PodSet.Pods[podID] = &store.Pod{
			PodID:      podID,
			StartedAt:  startedAt,
			Status:     status,
			Namespace:  ns,
			HasSynclet: hasSynclet,
		}
		return mt, ms.PodSet.Pods[podID]
	}

	podInfo, ok := ms.PodSet.Pods[podID]
	if !ok {
		// CASE 2: We have a set of pods for this DeployID, but not this particular pod -- record it
		podInfo = &store.Pod{
			PodID:      podID,
			StartedAt:  startedAt,
			Status:     status,
			Namespace:  ns,
			HasSynclet: hasSynclet,
		}
		ms.PodSet.Pods[podID] = podInfo
	}

	// CASE 3: This pod is already in the PodSet, nothing to do.
	return mt, podInfo
}

// Fill in container fields on the pod state.
func populateContainerStatus(ctx context.Context, manifest model.Manifest, podInfo *store.Pod, pod *v1.Pod, cStatus v1.ContainerStatus) {
	cName := k8s.ContainerNameFromContainerStatus(cStatus)
	podInfo.ContainerName = cName
	podInfo.ContainerReady = cStatus.Ready

	cID, err := k8s.ContainerIDFromContainerStatus(cStatus)
	if err != nil {
		logger.Get(ctx).Debugf("Error parsing container ID: %v", err)
		return
	}
	podInfo.ContainerID = cID

	cRef, err := container.ParseNamed(cStatus.Image)
	if err != nil {
		logger.Get(ctx).Debugf("Error parsing container image ID: %v", err)
		return
	}
	podInfo.ContainerImageRef = cRef

	ports := make([]int32, 0)
	cSpec := k8s.ContainerSpecOf(pod, cStatus)
	for _, cPort := range cSpec.Ports {
		ports = append(ports, cPort.ContainerPort)
	}
	podInfo.ContainerPorts = ports

	forwards := PopulatePortForwards(manifest, *podInfo)
	if len(forwards) < len(manifest.K8sTarget().PortForwards) {
		logger.Get(ctx).Infof(
			"WARNING: Resource %s is using port forwards, but no container ports on pod %s",
			manifest.Name, podInfo.PodID)
	}

	// HACK(maia): Go through ALL containers (except tilt-synclet), grab minimum info we need
	// to stream logs from them.
	var cInfos []store.ContainerInfo
	for _, cStat := range pod.Status.ContainerStatuses {
		if cStat.Name == sidecar.SyncletContainerName {
			// We don't want logs for the Tilt synclet.
			continue
		}

		cID, err := k8s.ContainerIDFromContainerStatus(cStat)
		if err != nil {
			logger.Get(ctx).Debugf("Error parsing container ID: %v", err)
			return
		}
		if err != nil {
			return
		}
		cInfos = append(cInfos, store.ContainerInfo{
			ID:   cID,
			Name: k8s.ContainerNameFromContainerStatus(cStat),
		})
	}
	podInfo.ContainerInfos = cInfos
}

func handleUIDUpdateAction(state *store.EngineState, action UIDUpdateAction) {
	switch action.EventType {
	case k8swatch.Added:
		state.ObjectsByK8SUIDs[action.UID] = store.UIDMapValue{
			Manifest: action.ManifestName,
			Entity:   action.Entity,
		}
	case k8swatch.Deleted:
		delete(state.ObjectsByK8SUIDs, action.UID)
	}
}

func handlePodChangeAction(ctx context.Context, state *store.EngineState, pod *v1.Pod) {
	mt, podInfo := ensureManifestTargetWithPod(state, pod)
	if mt == nil || podInfo == nil {
		return
	}

	ms := mt.State
	manifest := mt.Manifest
	podID := k8s.PodIDFromPod(pod)
	if podInfo.PodID != podID {
		// This is an event from an old pod.
		return
	}

	// Update the status
	podInfo.Deleting = pod.DeletionTimestamp != nil
	podInfo.Phase = pod.Status.Phase
	podInfo.Status = podStatusToString(*pod)

	defer prunePods(ms)

	// Check if the container is ready.
	var cStatus v1.ContainerStatus
	var err error
	if len(manifest.ImageTargets) > 0 {
		// Get status of (first) container matching (an) image we built for this manifest.
		for _, iTarget := range manifest.ImageTargets {
			cStatus, err = k8s.ContainerMatching(pod, container.NameSelector(iTarget.DeploymentRef))
			if err != nil {
				logger.Get(ctx).Debugf("Error matching container: %v", err)
				return
			}
			if cStatus.Name != "" {
				break
			}
		}
	} else {
		// We didn't build images for this manifest so we have no good way of figuring
		// out which container(s) we care about; for now, take the first.
		if len(pod.Status.ContainerStatuses) > 0 {
			cStatus = pod.Status.ContainerStatuses[0]
		}

	}

	if cStatus.Name == "" {
		return
	}

	populateContainerStatus(ctx, manifest, podInfo, pod, cStatus)
	checkForPodCrash(ctx, state, ms, *podInfo)

	if int(cStatus.RestartCount) > podInfo.ContainerRestarts {
		ms.CrashLog = podInfo.CurrentLog
		podInfo.CurrentLog = model.Log{}
	}
	podInfo.ContainerRestarts = int(cStatus.RestartCount)
}

func checkForPodCrash(ctx context.Context, state *store.EngineState, ms *store.ManifestState, podInfo store.Pod) {
	if ms.NeedsRebuildFromCrash {
		// We're already aware the pod is crashing.
		return
	}

	if ms.ExpectedContainerID == "" || ms.ExpectedContainerID == podInfo.ContainerID {
		// The pod is what we expect it to be.
		return
	}

	// The pod isn't what we expect!
	ms.CrashLog = podInfo.CurrentLog
	ms.NeedsRebuildFromCrash = true
	ms.ExpectedContainerID = ""
	msg := fmt.Sprintf("Detected a container change for %s. We could be running stale code. Rebuilding and deploying a new image.", ms.Name)
	le := store.NewLogEvent([]byte(msg + "\n"))
	if len(ms.BuildHistory) > 0 {
		ms.BuildHistory[0].Log = model.AppendLog(ms.BuildHistory[0].Log, le, state.LogTimestamps)
	}
	ms.CurrentBuild.Log = model.AppendLog(ms.CurrentBuild.Log, le, state.LogTimestamps)
	ms.CombinedLog = model.AppendLog(ms.CombinedLog, le, state.LogTimestamps)
	logger.Get(ctx).Infof("%s", msg)
}

// If there's more than one pod, prune the deleting/dead ones so
// that they don't clutter the output.
func prunePods(ms *store.ManifestState) {
	// Continue pruning until we have 1 pod.
	for ms.PodSet.Len() > 1 {
		bestPod := ms.MostRecentPod()

		for key, pod := range ms.PodSet.Pods {
			// Always remove pods that were manually deleted.
			if pod.Deleting {
				delete(ms.PodSet.Pods, key)
				break
			}

			// Remove terminated pods if they aren't the most recent one.
			isDead := pod.Phase == v1.PodSucceeded || pod.Phase == v1.PodFailed
			if isDead && pod.PodID != bestPod.PodID {
				delete(ms.PodSet.Pods, key)
				break
			}
		}

		// found nothing to delete, break out
		return
	}
}

func handlePodLogAction(state *store.EngineState, action PodLogAction) {
	manifestName := action.ManifestName
	ms, ok := state.ManifestState(manifestName)

	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	ms.CombinedLog = model.AppendLog(ms.CombinedLog, action, state.LogTimestamps)

	podID := action.PodID
	if !ms.PodSet.ContainsID(podID) {
		// NOTE(nick): There are two cases where this could happen:
		// 1) Pod 1 died and kubernetes started Pod 2. What should we do with
		//    logs from Pod 1 that are still in the action queue?
		//    This is an open product question. A future HUD may aggregate
		//    logs across pod restarts.
		// 2) Due to race conditions, we got the logs for Pod 1 before
		//    we saw Pod 1 materialize on the Pod API. The best way to fix
		//    this would be to make PodLogManager a subscriber that only
		//    starts listening on logs once the pod has materialized.
		//    We may prioritize this higher or lower based on how often
		//    this happens in practice.
		return
	}

	podInfo := ms.PodSet.Pods[podID]
	podInfo.CurrentLog = model.AppendLog(podInfo.CurrentLog, action, state.LogTimestamps)
}

func handleBuildLogAction(state *store.EngineState, action BuildLogAction) {
	manifestName := action.ManifestName
	ms, ok := state.ManifestState(manifestName)

	if !ok || state.CurrentlyBuilding != manifestName {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	ms.CombinedLog = model.AppendLog(ms.CombinedLog, action, state.LogTimestamps)
	ms.CurrentBuild.Log = model.AppendLog(ms.CurrentBuild.Log, action, state.LogTimestamps)
}

func handleLogAction(state *store.EngineState, action store.LogAction) {
	state.Log = model.AppendLog(state.Log, action, state.LogTimestamps)
}

func handleServiceEvent(ctx context.Context, state *store.EngineState, action ServiceChangeAction) {
	service := action.Service
	manifestName := model.ManifestName(service.ObjectMeta.Labels[k8s.ManifestNameLabel])
	if manifestName == "" || manifestName == model.UnresourcedYAMLManifestName {
		return
	}

	ms, ok := state.ManifestState(manifestName)
	if !ok {
		logger.Get(ctx).Infof("error: got notified of service for unknown manifest '%s'", manifestName)
		return
	}

	ms.LBs[k8s.ServiceName(service.Name)] = action.URL
}

func handleK8SEvent(ctx context.Context, state *store.EngineState, action store.K8SEventAction) {
	evt := action.Event
	v, ok := state.ObjectsByK8SUIDs[k8s.UID(evt.InvolvedObject.UID)]
	if !ok {
		return
	}

	if evt.Type != "Normal" {
		ms, ok := state.ManifestState(v.Manifest)
		if !ok {
			return
		}

		ms.CombinedLog = model.AppendLog(ms.CombinedLog, action, state.LogTimestamps)
		logger.Get(ctx).Infof("%s%s", logPrefix(ms.Name.String()), action.MessageRaw())

		ms.K8sWarnEvents = append(ms.K8sWarnEvents, k8s.NewEventWithEntity(evt, v.Entity))
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
	engineState.TriggerMode = action.TriggerMode
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
		logger.Get(ctx).Infof("event for unrecognized manifest %s", mn)
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
	manifestName := action.ManifestName
	ms, ok := state.ManifestState(manifestName)

	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	dcState, _ := ms.ResourceState.(dockercompose.State)
	ms.ResourceState = dcState.WithCurrentLog(model.AppendLog(dcState.CurrentLog, action, state.LogTimestamps))
	ms.CombinedLog = model.AppendLog(ms.CombinedLog, action, state.LogTimestamps)
}

func handleTiltfileLogAction(ctx context.Context, state *store.EngineState, action TiltfileLogAction) {
	state.CurrentTiltfileBuild.Log = model.AppendLog(state.CurrentTiltfileBuild.Log, action, state.LogTimestamps)
	state.TiltfileCombinedLog = model.AppendLog(state.TiltfileCombinedLog, action, state.LogTimestamps)
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
