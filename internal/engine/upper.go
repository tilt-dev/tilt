package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/tiltfile"
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
	kcli  k8s.Client
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

func NewUpper(ctx context.Context, hud hud.HeadsUpDisplay, pw *PodWatcher, sw *ServiceWatcher,
	st *store.Store, plm *PodLogManager, pfc *PortForwardController, fwm *WatchManager, bc *BuildController,
	ic *ImageController, gybc *GlobalYAMLBuildController, cc *ConfigsController,
	kcli k8s.Client, dcw *DockerComposeWatcher) Upper {

	st.AddSubscriber(bc)
	st.AddSubscriber(hud)
	st.AddSubscriber(pfc)
	st.AddSubscriber(plm)
	st.AddSubscriber(fwm)
	st.AddSubscriber(pw)
	st.AddSubscriber(sw)
	st.AddSubscriber(ic)
	st.AddSubscriber(gybc)
	st.AddSubscriber(cc)
	st.AddSubscriber(dcw)

	return Upper{
		store: st,
		kcli:  kcli,
	}
}

func (u Upper) Dispatch(action store.Action) {
	u.store.Dispatch(action)
}

func (u Upper) Start(ctx context.Context, args []string, watchMounts bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Start")
	defer span.Finish()

	err := u.kcli.ConnectedToCluster(ctx)
	if err != nil {
		return err
	}

	absTfPath, err := filepath.Abs(tiltfile.FileName)
	if err != nil {
		return err
	}

	manifestNames := make([]model.ManifestName, len(args))

	for i, a := range args {
		manifestNames[i] = model.ManifestName(a)
	}

	manifests, globalYAML, configFiles, err := loadAndGetManifests(ctx, manifestNames)
	// ~~ maybe this is just for now...?
	if err != nil {
		// TODO(dmiller): instead of returning, set the TiltfileError on state
		return err
	}

	u.store.Dispatch(InitAction{
		WatchMounts:        watchMounts,
		Manifests:          manifests,
		GlobalYAMLManifest: globalYAML,
		TiltfilePath:       absTfPath,
		ConfigFiles:        configFiles,
		ManifestNames:      manifestNames,
		Err:                err,
	})

	return u.store.Loop(ctx)
}

func loadAndGetManifests(ctx context.Context, manifestNames []model.ManifestName) (
	manifests []model.Manifest, globalYAML model.YAMLManifest, configFiles []string, err error) {

	tf, err := tiltfile.Load(ctx, tiltfile.FileName)
	if err != nil {
		return []model.Manifest{model.Manifest{Name: "Tiltfile"}}, model.YAMLManifest{}, []string{tiltfile.FileName}, err
	}

	return tf.GetManifestConfigsAndGlobalYAML(ctx, manifestNames...)
}

func (u Upper) StartForTesting(ctx context.Context, manifests []model.Manifest,
	globalYAML model.YAMLManifest, watchMounts bool, tiltfilePath string) error {

	manifestNames := make([]model.ManifestName, len(manifests))

	for i, m := range manifests {
		manifestNames[i] = m.ManifestName()
	}

	u.store.Dispatch(InitAction{
		WatchMounts:        watchMounts,
		Manifests:          manifests,
		GlobalYAMLManifest: globalYAML,
		TiltfilePath:       tiltfilePath,
		ManifestNames:      manifestNames,
	})

	return u.store.Loop(ctx)
}

var UpperReducer = store.Reducer(func(ctx context.Context, state *store.EngineState, action store.Action) {
	var err error
	switch action := action.(type) {
	case InitAction:
		err = handleInitAction(ctx, state, action)
	case ErrorAction:
		err = action.Error
	case hud.ExitAction:
		handleExitAction(state, action)
	case manifestFilesChangedAction:
		handleFSEvent(ctx, state, action)
	case PodChangeAction:
		handlePodEvent(ctx, state, action.Pod)
	case ServiceChangeAction:
		handleServiceEvent(ctx, state, action)
	case PodLogAction:
		handlePodLogAction(state, action)
	case BuildLogAction:
		handleBuildLogAction(state, action)
	case BuildCompleteAction:
		err = handleCompletedBuild(ctx, state, action)
	case BuildStartedAction:
		handleBuildStarted(ctx, state, action)
	case LogAction:
		handleLogAction(state, action)
	case GlobalYAMLApplyStartedAction:
		handleGlobalYAMLApplyStarted(ctx, state, action)
	case GlobalYAMLApplyCompleteAction:
		handleGlobalYAMLApplyComplete(ctx, state, action)
	case ConfigsReloadStartedAction:
		handleConfigsReloadStarted(ctx, state, action)
	case ConfigsReloadedAction:
		handleConfigsReloaded(ctx, state, action)
	case DockerComposeEventAction:
		handleDockerComposeEvent(ctx, state, action)
	default:
		err = fmt.Errorf("unrecognized action: %T", action)
	}

	if err != nil {
		state.PermanentError = err
	}
})

func handleBuildStarted(ctx context.Context, state *store.EngineState, action BuildStartedAction) {
	mn := action.Manifest.Name
	ms := state.ManifestStates[mn]

	ms.StartedFirstBuild = true
	ms.CurrentBuildEdits = append([]string{}, action.FilesChanged...)
	ms.CurrentBuildStartTime = action.StartTime
	ms.CurrentBuildReason = action.Reason
	for _, pod := range ms.PodSet.Pods {
		pod.CurrentLog = []byte{}
	}

	// Keep the crash log around until we have a rebuild
	// triggered by a explicit change (i.e., not a crash rebuild)
	if !action.Reason.IsCrashOnly() {
		ms.CrashLog = ""
	}

	state.CurrentlyBuilding = mn
}

func handleCompletedBuild(ctx context.Context, engineState *store.EngineState, cb BuildCompleteAction) error {
	defer func() {
		engineState.CurrentlyBuilding = ""
	}()

	engineState.CompletedBuildCount++
	engineState.BuildControllerActionCount++

	defer func() {
		if engineState.CompletedBuildCount == engineState.InitialBuildCount {
			logger.Get(ctx).Debugf("[timing.py] finished initial build") // hook for timing.py
		}
	}()

	err := cb.Error

	ms := engineState.ManifestStates[engineState.CurrentlyBuilding]

	startBuildTime := ms.CurrentBuildStartTime
	ms.LastBuildError = err
	ms.LastBuildStartTime = ms.CurrentBuildStartTime
	ms.LastBuildFinishTime = time.Now()
	ms.LastBuildDuration = time.Since(ms.CurrentBuildStartTime)
	ms.LastBuildReason = ms.CurrentBuildReason
	ms.LastBuildLog = ms.CurrentBuildLog

	ms.CurrentBuildStartTime = time.Time{}
	ms.CurrentBuildReason = model.BuildReasonNone
	ms.CurrentBuildLog = nil
	ms.NeedsRebuildFromCrash = false

	if err != nil {
		ms.CurrentBuildEdits = nil

		if isPermanentError(err) {
			return err
		} else if engineState.WatchMounts {
			l := logger.Get(ctx)
			p := logger.Red(l).Sprintf("Build Failed:")
			l.Infof("%s %v", p, err)
		} else {
			return errors.Wrap(err, "Build Failed")
		}
	} else {
		// Remove pending file changes that were consumed by this build.
		for file, modTime := range ms.PendingFileChanges {
			if modTime.Before(startBuildTime) {
				delete(ms.PendingFileChanges, file)
			}
		}

		if !ms.PendingManifestChange.IsZero() &&
			ms.PendingManifestChange.Before(startBuildTime) {
			ms.PendingManifestChange = time.Time{}
		}

		ms.LastSuccessfulDeployTime = time.Now()
		ms.LastBuild = cb.Result
		ms.LastSuccessfulDeployEdits = ms.CurrentBuildEdits
		ms.CurrentBuildEdits = nil

		for _, pod := range ms.PodSet.Pods {
			// # of pod restarts from old code (shouldn't be reflected in HUD)
			pod.OldRestarts = pod.ContainerRestarts
		}
	}

	if engineState.WatchMounts {
		logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py

		if cb.Result.ContainerID != "" {
			ms.ExpectedContainerID = cb.Result.ContainerID
		}
	}

	return nil
}

func handleFSEvent(
	ctx context.Context,
	state *store.EngineState,
	event manifestFilesChangedAction) {

	if event.manifestName == ConfigsManifestName {
		for _, f := range event.files {
			state.PendingConfigFileChanges[f] = true
		}
		return
	}

	ms := state.ManifestStates[event.manifestName]
	for _, f := range event.files {
		ms.PendingFileChanges[f] = time.Now()
	}
}

func handleGlobalYAMLApplyStarted(
	ctx context.Context,
	state *store.EngineState,
	event GlobalYAMLApplyStartedAction,
) {
	state.GlobalYAMLState.CurrentApplyStartTime = time.Now()
	state.GlobalYAMLState.LastError = nil
}

func handleGlobalYAMLApplyComplete(
	ctx context.Context,
	state *store.EngineState,
	event GlobalYAMLApplyCompleteAction,
) {
	ms := state.GlobalYAMLState
	ms.LastApplyFinishTime = time.Now()
	ms.LastApplyDuration = time.Since(ms.CurrentApplyStartTime)
	ms.CurrentApplyStartTime = time.Time{}

	ms.LastError = event.Error

	if event.Error == nil {
		ms.HasBeenDeployed = true
		ms.LastSuccessfulApplyTime = time.Now()
	}
}

func handleConfigsReloadStarted(
	ctx context.Context,
	state *store.EngineState,
	event ConfigsReloadStartedAction,
) {
	state.PendingConfigFileChanges = make(map[string]bool)
}

func handleConfigsReloaded(
	ctx context.Context,
	state *store.EngineState,
	event ConfigsReloadedAction,
) {
	manifests := event.Manifests
	err := event.Err
	if err != nil {
		logger.Get(ctx).Infof("Unable to parse Tiltfile: %v", err)
		state.LastTiltfileError = err
		return
	}
	newDefOrder := make([]model.ManifestName, len(manifests))
	for i, m := range manifests {
		ms, ok := state.ManifestStates[m.ManifestName()]
		if !ok {
			ms = &store.ManifestState{}
		}

		newDefOrder[i] = m.ManifestName()
		if !m.Equal(ms.Manifest) {
			ms.Manifest = m

			// Manifest has changed, ensure we do an image build so that we apply the changes
			ms.LastBuild = store.BuildResult{}
			ms.PendingManifestChange = time.Now()
		}
		state.ManifestStates[m.ManifestName()] = ms
	}
	// TODO(dmiller) handle deleting manifests
	// TODO(maia): update ConfigsManifest with new ConfigFiles/update watches
	state.ManifestDefinitionOrder = newDefOrder
	state.GlobalYAML = event.GlobalYAML
	state.ConfigFiles = event.ConfigFiles
	state.LastTiltfileError = nil
}

// Get a pointer to a mutable manifest state,
// ensuring that some Pod exists on the state.
//
// Intended as a helper for pod-mutating events.
func ensureManifestStateWithPod(state *store.EngineState, pod *v1.Pod) (*store.ManifestState, *store.Pod) {
	manifestName := model.ManifestName(pod.ObjectMeta.Labels[ManifestNameLabel])
	if manifestName == "" {
		return nil, nil
	}

	podID := k8s.PodIDFromPod(pod)
	startedAt := pod.CreationTimestamp.Time
	status := podStatusToString(*pod)
	ns := k8s.NamespaceFromPod(pod)

	ms, ok := state.ManifestStates[manifestName]
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return nil, nil
	}

	imageID, err := k8s.FindImageNamedTaggedMatching(pod.Spec, ms.Manifest.DockerRef())
	if err != nil || imageID == nil {
		// Ditto, this could happen if we get a pod from an old version of the manifest.
		return nil, nil
	}

	// There are 4 cases:
	// 1) This pod has an imageID we don't recognize because it's an old build
	// 2) This pod has an imageID we don't recognize because it's a new build
	// 3) This pod has an imageID we recognize, and we need to record it.
	// 4) This pod has an imageID we recognize, and we've already recorded it.

	// (1) + (2)
	if ms.PodSet.ImageID == nil ||
		ms.PodSet.ImageID.String() != imageID.String() {

		bestPod := ms.MostRecentPod()
		isOld := !bestPod.Empty() && bestPod.StartedAt.After(startedAt)
		if isOld {
			// (1)
			return nil, nil
		}

		// (2)
		ms.PodSet = store.PodSet{
			ImageID: imageID,
			Pods:    make(map[k8s.PodID]*store.Pod),
		}
		ms.PodSet.Pods[podID] = &store.Pod{
			PodID:     podID,
			StartedAt: startedAt,
			Status:    status,
			Namespace: ns,
		}
		return ms, ms.PodSet.Pods[podID]
	}

	podInfo, ok := ms.PodSet.Pods[podID]
	if !ok {
		// (3)
		podInfo = &store.Pod{
			PodID:     podID,
			StartedAt: startedAt,
			Status:    status,
			Namespace: ns,
		}
		ms.PodSet.Pods[podID] = podInfo
	}

	// (4)
	return ms, podInfo
}

// Fill in container fields on the pod state.
func populateContainerStatus(ctx context.Context, ms *store.ManifestState, podInfo *store.Pod, pod *v1.Pod, cStatus v1.ContainerStatus) {
	cName := k8s.ContainerNameFromContainerStatus(cStatus)
	podInfo.ContainerName = cName
	podInfo.ContainerReady = cStatus.Ready

	cID, err := k8s.ContainerIDFromContainerStatus(cStatus)
	if err != nil {
		logger.Get(ctx).Debugf("Error parsing container ID: %v", err)
		return
	}
	podInfo.ContainerID = cID

	ports := make([]int32, 0)
	cSpec := k8s.ContainerSpecOf(pod, cStatus)
	for _, cPort := range cSpec.Ports {
		ports = append(ports, cPort.ContainerPort)
	}
	podInfo.ContainerPorts = ports

	forwards := PopulatePortForwards(ms.Manifest, *podInfo)
	if len(forwards) < len(ms.Manifest.PortForwards()) {
		logger.Get(ctx).Infof(
			"WARNING: Resource %s is using port forwards, but no container ports on pod %s",
			ms.Manifest.Name, podInfo.PodID)
	}
}

func handlePodEvent(ctx context.Context, state *store.EngineState, pod *v1.Pod) {
	ms, podInfo := ensureManifestStateWithPod(state, pod)
	if ms == nil || podInfo == nil {
		return
	}

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
	cStatus, err := k8s.ContainerMatching(pod, ms.Manifest.DockerRef())
	if err != nil {
		logger.Get(ctx).Debugf("Error matching container: %v", err)
		return
	} else if cStatus.Name == "" {
		return
	}

	populateContainerStatus(ctx, ms, podInfo, pod, cStatus)
	if ms.ExpectedContainerID != "" && ms.ExpectedContainerID != podInfo.ContainerID && !ms.NeedsRebuildFromCrash {
		ms.CrashLog = string(podInfo.CurrentLog)
		ms.NeedsRebuildFromCrash = true
		ms.ExpectedContainerID = ""
		logger.Get(ctx).Infof("Detected a container change for %s. We could be running stale code. Rebuilding and deploying a new image.", ms.Manifest.Name)
	}

	if int(cStatus.RestartCount) > podInfo.ContainerRestarts {
		podInfo.PreRestartLog = append([]byte{}, podInfo.CurrentLog...)
		podInfo.CurrentLog = []byte{}
	}
	podInfo.ContainerRestarts = int(cStatus.RestartCount)
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
	ms, ok := state.ManifestStates[manifestName]

	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}

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
	podInfo.CurrentLog = append(podInfo.CurrentLog, action.Log...)
}

func handleBuildLogAction(state *store.EngineState, action BuildLogAction) {
	manifestName := action.ManifestName
	ms, ok := state.ManifestStates[manifestName]

	if !ok || state.CurrentlyBuilding != manifestName {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	ms.CurrentBuildLog = append(ms.CurrentBuildLog, action.Log...)
}

func handleLogAction(state *store.EngineState, action LogAction) {
	state.Log = append(state.Log, action.Log...)
}

func handleServiceEvent(ctx context.Context, state *store.EngineState, action ServiceChangeAction) {
	service := action.Service
	manifestName := model.ManifestName(service.ObjectMeta.Labels[ManifestNameLabel])
	if manifestName == "" || manifestName == model.GlobalYAMLManifestName {
		return
	}

	ms, ok := state.ManifestStates[manifestName]
	if !ok {
		logger.Get(ctx).Infof("error: got notified of service for unknown manifest '%s'", manifestName)
		return
	}

	ms.LBs[k8s.ServiceName(service.Name)] = action.URL
}

func handleInitAction(ctx context.Context, engineState *store.EngineState, action InitAction) error {
	engineState.TiltfilePath = action.TiltfilePath
	engineState.ConfigFiles = action.ConfigFiles
	engineState.InitManifests = action.ManifestNames
	watchMounts := action.WatchMounts
	manifests := action.Manifests

	engineState.GlobalYAML = action.GlobalYAMLManifest
	engineState.GlobalYAMLState = store.NewYAMLManifestState()

	for _, m := range manifests {
		engineState.ManifestDefinitionOrder = append(engineState.ManifestDefinitionOrder, m.Name)
		engineState.ManifestStates[m.Name] = store.NewManifestState(m)
	}
	engineState.WatchMounts = watchMounts

	engineState.InitialBuildCount = len(manifests)
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
	ms, ok := engineState.ManifestStates[model.ManifestName(mn)]
	if !ok {
		// No corresponding manifest, nothing to do
		logger.Get(ctx).Infof("event for unrecognized manifest %s", mn)
		return
	}

	// For now, just guess at state.
	state, ok := evt.GuessState()
	logger.Get(ctx).Infof("guessing state for %s event: %s", evt.Type, evt.Action)
	if ok {
		logger.Get(ctx).Infof("state is probably: %s", state)
		ms.DCInfo.State = state
	}
}

// Check if the filesChangedSet only contains spurious changes that
// we don't want to rebuild on, like IDE temp/lock files.
//
// NOTE(nick): This isn't an ideal solution. In an ideal world, the user would
// put everything to ignore in their gitignore/dockerignore files. This is a stop-gap
// so they don't have a terrible experience if those files aren't there or
// aren't in the right places.
func onlySpuriousChanges(filesChanged map[string]time.Time) (bool, error) {
	// If a lot of files have changed, don't treat this as spurious.
	if len(filesChanged) > 3 {
		return false, nil
	}

	for f := range filesChanged {
		broken, err := ospath.IsBrokenSymlink(f)
		if err != nil {
			return false, err
		}

		if !broken {
			return false, nil
		}
	}
	return true, nil
}
