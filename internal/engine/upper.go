package engine

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/opentracing/opentracing-go"
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
	b     BuildAndDeployer
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

func NewUpper(ctx context.Context, b BuildAndDeployer,
	hud hud.HeadsUpDisplay, pw *PodWatcher, sw *ServiceWatcher,
	st *store.Store, plm *PodLogManager, pfc *PortForwardController,
	fwm *WatchManager, fswm FsWatcherMaker, bc *BuildController,
	ic *ImageController, gybc *GlobalYAMLBuildController, tfw *TiltfileWatcher) Upper {

	st.AddSubscriber(bc)
	st.AddSubscriber(hud)
	st.AddSubscriber(pfc)
	st.AddSubscriber(plm)
	st.AddSubscriber(fwm)
	st.AddSubscriber(pw)
	st.AddSubscriber(sw)
	st.AddSubscriber(ic)
	st.AddSubscriber(gybc)
	st.AddSubscriber(tfw)

	return Upper{
		b:     b,
		store: st,
	}
}

func (u Upper) Dispatch(action store.Action) {
	u.store.Dispatch(action)
}

func (u Upper) Start(ctx context.Context, args []string, watchMounts bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Start")
	defer span.Finish()

	tf, err := tiltfile.Load(ctx, tiltfile.FileName)
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

	manifests, globalYAML, configFiles, err := tf.GetManifestConfigsAndGlobalYAML(ctx, manifestNames...)
	if err != nil {
		return err
	}

	u.store.Dispatch(InitAction{
		WatchMounts:        watchMounts,
		Manifests:          manifests,
		GlobalYAMLManifest: globalYAML,
		TiltfilePath:       absTfPath,
		ConfigFiles:        configFiles,
		ManifestNames:      manifestNames,
	})

	return u.store.Loop(ctx)
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
	case BuildCompleteAction:
		err = handleCompletedBuild(ctx, state, action)
	case BuildStartedAction:
		handleBuildStarted(ctx, state, action)
	// case ManifestReloadedAction:
	// 	handleManifestReloaded(ctx, state, action)
	case LogAction:
		handleLogAction(state, action)
	// case GlobalYAMLManifestReloadedAction:
	// 	handleGlobalYAMLManifestReloaded(ctx, state, action)
	case GlobalYAMLApplyStartedAction:
		handleGlobalYAMLApplyStarted(ctx, state, action)
	case GlobalYAMLApplyCompleteAction:
		handleGlobalYAMLApplyComplete(ctx, state, action)
	case GlobalYAMLApplyError:
		handleGlobalYAMLApplyError(ctx, state, action)
	case TiltfileReloadStartedAction:
		handleTiltfileReloadStarted(ctx, state, action)
	case TiltfileReloadedAction:
		handleTiltfileReloaded(ctx, state, action)
	default:
		err = fmt.Errorf("unrecognized action: %T", action)
	}

	if err != nil {
		state.PermanentError = err
	}
})

// func handleManifestReloaded(ctx context.Context, state *store.EngineState, action ManifestReloadedAction) {
// 	state.BuildControllerActionCount++

// 	ms, ok := state.ManifestStates[action.OldManifest.Name]
// 	if !ok {
// 		state.PermanentError = fmt.Errorf("handleManifestReloaded: Missing manifest state: %s", action.OldManifest.Name)
// 		return
// 	}

// 	ms.LastManifestLoadError = action.Error
// 	if ms.LastManifestLoadError != nil {
// 		logger.Get(ctx).Infof("getting new manifest error: %v", ms.LastManifestLoadError)

// 		err := removeFromManifestsToBuild(state, ms.Manifest.Name)
// 		if err != nil {
// 			state.PermanentError = fmt.Errorf("handleManifestReloaded: %v", err)
// 			return
// 		}
// 		return
// 	}

// 	newManifest := action.NewManifest
// 	if newManifest.Equal(ms.Manifest) {
// 		logger.Get(ctx).Debugf("Detected config change, but manifest %s hasn't changed",
// 			ms.Manifest.Name)

// 		mountedChangedFiles, err := ms.PendingFileChangesWithoutUnmountedConfigFiles(ctx)
// 		if err != nil {
// 			logger.Get(ctx).Infof(err.Error())
// 			return
// 		}
// 		ms.PendingFileChanges = mountedChangedFiles

// 		if len(ms.PendingFileChanges) == 0 {
// 			ms.ConfigIsDirty = false
// 			err = removeFromManifestsToBuild(state, ms.Manifest.Name)
// 			if err != nil {
// 				state.PermanentError = fmt.Errorf("handleManifestReloaded: %v", err)
// 			}
// 			return
// 		}
// 	} else {
// 		// Manifest has changed, ensure we do an image build so that we apply the changes
// 		ms.LastBuild = store.BuildResult{}
// 		ms.Manifest = newManifest
// 	}

// 	ms.ConfigIsDirty = false
// }

func removeFromManifestsToBuild(state *store.EngineState, mn model.ManifestName) error {
	for i, n := range state.ManifestsToBuild {
		if n == mn {
			state.ManifestsToBuild = append(state.ManifestsToBuild[:i], state.ManifestsToBuild[i+1:]...)
			state.ManifestStates[mn].QueueEntryTime = time.Time{}
			return nil
		}
	}

	return fmt.Errorf("Missing manifest %s", mn)
}

func handleBuildStarted(ctx context.Context, state *store.EngineState, action BuildStartedAction) {
	mn := action.Manifest.Name
	err := removeFromManifestsToBuild(state, mn)
	if err != nil {
		state.PermanentError = fmt.Errorf("handleBuildStarted: %v", err)
		return
	}

	ms := state.ManifestStates[mn]

	ms.CurrentlyBuildingFileChanges = append([]string{}, action.FilesChanged...)
	for _, file := range action.FilesChanged {
		delete(ms.PendingFileChanges, file)
	}
	ms.CurrentBuildStartTime = action.StartTime
	ms.Pod.CurrentLog = []byte{}

	// TODO(nick): It would be better if we reversed the relationship
	// between CurrentlyBuilding and BuildController. BuildController should dispatch
	// a StartBuildAction, and that should change the state of CurrentlyBuilding
	// (rather than BuildController starting in response to CurrentlyBuilding).
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
	ms.HasBeenBuilt = true
	ms.LastBuildError = err
	ms.LastBuildFinishTime = time.Now()
	ms.LastBuildDuration = time.Since(ms.CurrentBuildStartTime)
	ms.CurrentBuildStartTime = time.Time{}
	ms.LastBuildLog = ms.CurrentBuildLog
	ms.CurrentBuildLog = &bytes.Buffer{}
	ms.CrashRebuildInProg = false

	if err != nil {
		// Put the files that failed to build back into the pending queue.
		for _, file := range ms.CurrentlyBuildingFileChanges {
			ms.PendingFileChanges[file] = true
		}
		ms.CurrentlyBuildingFileChanges = nil

		if isPermanentError(err) {
			return err
		} else if engineState.WatchMounts {
			l := logger.Get(ctx)
			p := logger.Red(l).Sprintf("Build Failed:")
			l.Infof("%s %v", p, err)
		} else {
			return fmt.Errorf("Build Failed: %v", err)
		}
	} else {
		ms.LastSuccessfulDeployTime = time.Now()
		ms.LastBuild = cb.Result
		ms.LastSuccessfulDeployEdits = ms.CurrentlyBuildingFileChanges
		ms.CurrentlyBuildingFileChanges = nil

		ms.Pod.OldRestarts = ms.Pod.ContainerRestarts // # of pod restarts from old code (shouldn't be reflected in HUD)
	}

	if engineState.WatchMounts {
		logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py

		if len(engineState.ManifestsToBuild) == 0 {
			l := logger.Get(ctx)
			l.Infof("%s", logger.Green(l).Sprintf("Awaiting changes…\n"))
		}

		if cb.Result.ContainerID != "" {
			if ms, ok := engineState.ManifestStates[ms.Manifest.Name]; ok {
				ms.ExpectedContainerID = cb.Result.ContainerID
			}
		}
	}

	return nil
}

func handleFSEvent(
	ctx context.Context,
	state *store.EngineState,
	event manifestFilesChangedAction) {

	if event.manifestName == "Tiltfile" {
		for _, f := range event.files {
			state.PendingConfigFileChanges[f] = true
		}
		return
	}

	// if eventContainsConfigFiles(manifest, event) {
	// 	logger.Get(ctx).Debugf("Event contains config files")
	// 	state.ManifestStates[event.manifestName].ConfigIsDirty = true
	// }

	ms := state.ManifestStates[event.manifestName]

	for _, f := range event.files {
		ms.PendingFileChanges[f] = true
	}

	spurious, err := onlySpuriousChanges(ms.PendingFileChanges)
	if err != nil {
		logger.Get(ctx).Infof("build watch error: %v", err)
	}

	if spurious {
		// TODO(nick): I think we probably want to log when this happens?
		return
	}

	// if the name is already in the queue, we don't need to add it again
	for _, mn := range state.ManifestsToBuild {
		if mn == event.manifestName {
			return
		}
	}

	enqueueBuild(state, event.manifestName)
}

// func handleGlobalYAMLManifestReloaded(
// 	ctx context.Context,
// 	state *store.EngineState,
// 	event GlobalYAMLManifestReloadedAction,
// ) {
// 	state.GlobalYAML = event.GlobalYAML
// }

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
	ms.HasBeenDeployed = true
	ms.LastApplyFinishTime = time.Now()
	ms.LastApplyDuration = time.Since(ms.CurrentApplyStartTime)
	ms.CurrentApplyStartTime = time.Time{}

	ms.LastSuccessfulApplyTime = time.Now()
	ms.LastError = nil
}

func handleGlobalYAMLApplyError(
	ctx context.Context,
	state *store.EngineState,
	event GlobalYAMLApplyError,
) {
	state.GlobalYAMLState.LastError = event.Error
}

func handleTiltfileReloadStarted(
	ctx context.Context,
	state *store.EngineState,
	event TiltfileReloadStartedAction,
) {
	state.PendingConfigFileChanges = make(map[string]bool)
}

func handleTiltfileReloaded(
	ctx context.Context,
	state *store.EngineState,
	event TiltfileReloadedAction,
) {
	manifests := event.Manifests
	globalYAML := event.GlobalYAML
	err := event.Err
	if err != nil {
		logger.Get(ctx).Infof("Unable to parse Tiltfile: %v", err)

		for _, ms := range state.ManifestStates {
			ms.LastManifestLoadError = err
		}
		return
	}
	newDefOrder := make([]model.ManifestName, len(manifests))
	for i, m := range manifests {
		ms, ok := state.ManifestStates[m.ManifestName()]
		if !ok {
			ms = &store.ManifestState{}
		}
		ms.LastManifestLoadError = nil
		state.ManifestStates[m.ManifestName()].LastManifestLoadError = nil

		newDefOrder[i] = m.ManifestName()
		if !m.Equal(ms.Manifest) {
			// Manifest has changed, ensure we do an image build so that we apply the changes
			ms.LastBuild = store.BuildResult{}
			ms.Manifest = m
			// TODO(dbentley): change watches when manifest changes
			// TODO(dbentley): add changed file to pending file changes?
			enqueueBuild(state, m.ManifestName())
		}
		state.ManifestStates[m.ManifestName()] = ms
	}
	// TODO(dmiller) handle deleting manifests
	state.ManifestDefinitionOrder = newDefOrder
	state.GlobalYAML = globalYAML
}

func enqueueBuild(state *store.EngineState, mn model.ManifestName) {
	state.ManifestsToBuild = append(state.ManifestsToBuild, mn)
	state.ManifestStates[mn].QueueEntryTime = time.Now()
}

// Get a pointer to a mutable manifest state,
// ensuring that some Pod exists on the state.
//
// Intended as a helper for pod-mutating events.
func ensureManifestStateWithPod(state *store.EngineState, pod *v1.Pod) *store.ManifestState {
	manifestName := model.ManifestName(pod.ObjectMeta.Labels[ManifestNameLabel])
	if manifestName == "" {
		return nil
	}

	podID := k8s.PodIDFromPod(pod)
	startedAt := pod.CreationTimestamp.Time
	status := podStatusToString(*pod)
	ns := k8s.NamespaceFromPod(pod)

	ms, ok := state.ManifestStates[manifestName]
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return nil
	}

	// If the pod is empty, or older then the current pod, replace it.
	if ms.Pod.PodID == "" || ms.Pod.StartedAt.Before(startedAt) {
		ms.Pod = store.Pod{
			PodID:     podID,
			StartedAt: startedAt,
			Status:    status,
			Namespace: ns,
		}
	}

	return ms
}

// Fill in container fields on the pod state.
func populateContainerStatus(ctx context.Context, ms *store.ManifestState, pod *v1.Pod, cStatus v1.ContainerStatus) {
	cName := k8s.ContainerNameFromContainerStatus(cStatus)
	ms.Pod.ContainerName = cName
	ms.Pod.ContainerReady = cStatus.Ready

	cID, err := k8s.ContainerIDFromContainerStatus(cStatus)
	if err != nil {
		logger.Get(ctx).Debugf("Error parsing container ID: %v", err)
		return
	}
	ms.Pod.ContainerID = cID

	ports := make([]int32, 0)
	cSpec := k8s.ContainerSpecOf(pod, cStatus)
	for _, cPort := range cSpec.Ports {
		ports = append(ports, cPort.ContainerPort)
	}
	ms.Pod.ContainerPorts = ports

	forwards := PopulatePortForwards(ms.Manifest, ms.Pod)
	if len(forwards) < len(ms.Manifest.PortForwards()) {
		logger.Get(ctx).Infof(
			"WARNING: Resource %s is using port forwards, but no container ports on pod %s",
			ms.Manifest.Name, ms.Pod.PodID)
	}
}

func handlePodEvent(ctx context.Context, state *store.EngineState, pod *v1.Pod) {
	ms := ensureManifestStateWithPod(state, pod)
	if ms == nil {
		return
	}

	podID := k8s.PodIDFromPod(pod)
	if ms.Pod.PodID != podID {
		// This is an event from an old pod.
		return
	}

	// Update the status
	ms.Pod.Phase = pod.Status.Phase
	ms.Pod.Status = podStatusToString(*pod)

	// Check if the container is ready.
	cStatus, err := k8s.ContainerMatching(pod, ms.Manifest.DockerRef())
	if err != nil {
		logger.Get(ctx).Debugf("Error matching container: %v", err)
		return
	} else if cStatus.Name == "" {
		return
	}

	populateContainerStatus(ctx, ms, pod, cStatus)
	if ms.ExpectedContainerID != "" && ms.ExpectedContainerID != ms.Pod.ContainerID && !ms.CrashRebuildInProg {
		ms.CrashRebuildInProg = true
		ms.ExpectedContainerID = ""
		logger.Get(ctx).Infof("Detected a container change for %s. We could be running stale code. Rebuilding and deploying a new image.", ms.Manifest.Name)
		enqueueBuild(state, ms.Manifest.Name)
	}

	if int(cStatus.RestartCount) > ms.Pod.ContainerRestarts {
		ms.Pod.PreRestartLog = append([]byte{}, ms.Pod.CurrentLog...)
		ms.Pod.CurrentLog = []byte{}
	}
	ms.Pod.ContainerRestarts = int(cStatus.RestartCount)
}

func handlePodLogAction(state *store.EngineState, action PodLogAction) {
	manifestName := action.ManifestName
	ms, ok := state.ManifestStates[manifestName]

	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	if ms.Pod.PodID != action.PodID {
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

	ms.Pod.CurrentLog = append(ms.Pod.CurrentLog, action.Log...)
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
	engineState.GlobalYAMLState = store.NewYAMLManifestState(action.GlobalYAMLManifest)

	for _, m := range manifests {
		engineState.ManifestDefinitionOrder = append(engineState.ManifestDefinitionOrder, m.Name)
		engineState.ManifestStates[m.Name] = store.NewManifestState(m)
	}
	engineState.WatchMounts = watchMounts

	for _, m := range manifests {
		enqueueBuild(engineState, m.Name)
	}
	engineState.InitialBuildCount = len(engineState.ManifestsToBuild)
	return nil
}

func handleExitAction(state *store.EngineState, action hud.ExitAction) {
	if action.Err != nil {
		state.PermanentError = action.Err
	} else {
		state.UserExited = true
	}
}

// Check if the filesChangedSet only contains spurious changes that
// we don't want to rebuild on, like IDE temp/lock files.
//
// NOTE(nick): This isn't an ideal solution. In an ideal world, the user would
// put everything to ignore in their gitignore/dockerignore files. This is a stop-gap
// so they don't have a terrible experience if those files aren't there or
// aren't in the right places.
func onlySpuriousChanges(filesChanged map[string]bool) (bool, error) {
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

// type configFilesManifest interface {
// 	ConfigMatcher() (model.PathMatcher, error)
// }

// func eventContainsConfigFiles(manifest configFilesManifest, e manifestFilesChangedAction) bool {
// 	matcher, err := manifest.ConfigMatcher()
// 	if err != nil {
// 		return false
// 	}

// 	for _, f := range e.files {
// 		matches, err := matcher.Matches(f, false)
// 		if matches && err == nil {
// 			return true
// 		}
// 	}

// 	return false
// }
