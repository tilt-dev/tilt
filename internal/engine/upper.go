package engine

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/build"
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
	fsWatcherMaker      fsWatcherMaker
	timerMaker          timerMaker
	podWatcherMaker     PodWatcherMaker
	serviceWatcherMaker ServiceWatcherMaker
	k8s                 k8s.Client
	reaper              build.ImageReaper
	hud                 hud.HeadsUpDisplay
	store               *store.Store
	hudErrorCh          chan error
}

type fsWatcherMaker func() (watch.Notify, error)
type ServiceWatcherMaker func(context.Context, *store.Store) error
type PodWatcherMaker func(context.Context, *store.Store) error
type timerMaker func(d time.Duration) <-chan time.Time

func ProvidePodWatcherMaker(kCli k8s.Client) PodWatcherMaker {
	return func(ctx context.Context, store *store.Store) error {
		return makePodWatcher(ctx, kCli, store)
	}
}

func ProvideServiceWatcherMaker(kCli k8s.Client) ServiceWatcherMaker {
	return func(ctx context.Context, store *store.Store) error {
		return makeServiceWatcher(ctx, kCli, store)
	}
}

func NewUpper(ctx context.Context, k8s k8s.Client,
	reaper build.ImageReaper, hud hud.HeadsUpDisplay, pwm PodWatcherMaker, swm ServiceWatcherMaker,
	st *store.Store, plm *PodLogManager, pfc *PortForwardController, bc *BuildController) Upper {
	fsWatcherMaker := func() (watch.Notify, error) {
		return watch.NewWatcher()
	}

	st.AddSubscriber(bc)
	st.AddSubscriber(hud)
	st.AddSubscriber(pfc)
	st.AddSubscriber(plm)

	return Upper{
		fsWatcherMaker:      fsWatcherMaker,
		podWatcherMaker:     pwm,
		serviceWatcherMaker: swm,
		timerMaker:          time.After,
		k8s:                 k8s,
		reaper:              reaper,
		hud:                 hud,
		store:               st,
		hudErrorCh:          make(chan error),
	}
}

func (u Upper) RunHud(ctx context.Context) error {
	err := u.hud.Run(ctx, u.store, hud.DefaultRefreshInterval)
	u.hudErrorCh <- err
	close(u.hudErrorCh)
	return err
}

func (u Upper) CreateManifests(ctx context.Context, manifests []model.Manifest, watchMounts bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-Up")
	defer span.Finish()

	u.store.Dispatch(InitAction{
		WatchMounts: watchMounts,
		Manifests:   manifests,
	})

	defer func() {
		u.hud.Close()
		// make sure the hud has had a chance to clean up
		<-u.hudErrorCh
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

			// Reducers
		case action := <-u.store.Actions():
			state := u.store.LockMutableState()
			err := u.reduceAction(ctx, state, action)
			if err != nil {
				state.PermanentError = err
			}
			handleMaybeStartBuild(ctx, state)
			u.store.UnlockMutableState()
		}

		// Subscribers
		done, err := maybeFinished(u.store)
		if done {
			return err
		}
		u.store.NotifySubscribers(ctx)
	}
}

func (u Upper) reduceAction(ctx context.Context, state *store.EngineState, action store.Action) error {
	switch action := action.(type) {
	case InitAction:
		return u.handleInitAction(ctx, state, action)
	case ErrorAction:
		return action.Error
	case manifestFilesChangedAction:
		handleFSEvent(ctx, state, action)
	case PodChangeAction:
		handlePodEvent(ctx, state, action.Pod)
	case ServiceChangeAction:
		handleServiceEvent(ctx, state, action.Service)
	case PodLogAction:
		handlePodLogAction(state, action)
	case BuildCompleteAction:
		return handleCompletedBuild(ctx, state, action)
	case hud.ShowErrorAction:
		showError(ctx, state, action.ResourceNumber)
	default:
		return fmt.Errorf("Unrecognized action: %T", action)
	}
	return nil
}

func maybeFinished(st *store.Store) (bool, error) {
	state := st.RLockState()
	defer st.RUnlockState()

	if len(state.ManifestStates) == 0 {
		return false, nil
	}

	if state.PermanentError != nil {
		return true, state.PermanentError
	}

	finished := !state.WatchMounts && len(state.ManifestsToBuild) == 0 && state.CurrentlyBuilding == ""
	return finished, nil
}

func handleMaybeStartBuild(ctx context.Context, state *store.EngineState) {
	if len(state.ManifestStates) == 0 ||
		len(state.ManifestsToBuild) == 0 ||
		state.CurrentlyBuilding != "" {
		return
	}

	mn := state.ManifestsToBuild[0]
	state.ManifestsToBuild = state.ManifestsToBuild[1:]
	ms := state.ManifestStates[mn]
	ms.QueueEntryTime = time.Time{}

	if ms.ConfigIsDirty {
		// TODO(nick): Move this into BuildController, so that
		// we don't hold the write lock on store while we reevaluate
		// the tiltfile.
		newManifest, err := getNewManifestFromTiltfile(mn)
		if err != nil {
			logger.Get(ctx).Infof("getting new manifest error: %v", err)
			ms.LastError = err
			ms.LastBuildFinishTime = time.Now()
			ms.LastBuildDuration = 0
			return
		}

		if newManifest.Equal(ms.Manifest) {
			logger.Get(ctx).Debugf("Detected config change, but manifest %s hasn't changed",
				ms.Manifest.Name)

			if _, ok := ms.LastError.(*manifestErr); ok {
				// Last err indicates failure to make a new manifest b/c of bad config files.
				// Manifest is now back to normal (the new one we just got is the same as the
				// one we previously had) so clear this error.
				ms.LastError = nil
			}

			mountedFilesChangedSinceLastSuccessfulBuild, err := ms.WithoutUnmountedConfigFiles(ctx, ms.FileChangesSinceLastSuccessfulBuild)
			if err != nil {
				logger.Get(ctx).Infof("error determining whether files are unmounted config files: %v", err)
				return
			}

			if len(mountedFilesChangedSinceLastSuccessfulBuild) == 0 {
				// No mounted files changed, no need to build.
				ms.ConfigIsDirty = false
				return
			}

		} else {
			// Manifest has changed, ensure we do an image build so that we apply the changes
			ms.LastBuild = store.BuildResult{}
			ms.Manifest = newManifest
		}

		ms.ConfigIsDirty = false
	}

	for f := range ms.FileChangesSinceLastBuild {
		ms.NewFileChangesInCurrentBuild = append(ms.NewFileChangesInCurrentBuild, f)
	}
	ms.FileChangesSinceLastBuild = make(map[string]bool)
	ms.CurrentBuildStartTime = time.Now()
	ms.Pod.Log = []byte{}

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

	defer func() {
		if engineState.CompletedBuildCount == engineState.InitialBuildCount {
			logger.Get(ctx).Debugf("[timing.py] finished initial build") // hook for timing.py
		}
	}()

	err := cb.Error

	ms := engineState.ManifestStates[engineState.CurrentlyBuilding]
	ms.HasBeenBuilt = true
	ms.LastError = err
	ms.LastBuildFinishTime = time.Now()
	ms.LastBuildDuration = time.Since(ms.CurrentBuildStartTime)
	ms.CurrentBuildStartTime = time.Time{}
	ms.LastBuildLog = ms.CurrentBuildLog
	ms.CurrentBuildLog = &bytes.Buffer{}
	ms.FileChangesSinceLastBuild = make(map[string]bool)

	if err != nil {
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
		ms.LastSuccessfulDeployEdits = ms.NewFileChangesInCurrentBuild
		ms.FileChangesSinceLastSuccessfulBuild = make(map[string]bool)
		ms.Pod.OldRestarts = ms.Pod.ContainerRestarts // # of pod restarts from old code (shouldn't be reflected in HUD)
	}

	ms.NewFileChangesInCurrentBuild = nil

	if engineState.WatchMounts {
		logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py

		if len(engineState.ManifestsToBuild) == 0 {
			l := logger.Get(ctx)
			l.Infof("%s", logger.Green(l).Sprintf("Awaiting changes…\n"))
		}
	}

	return nil
}

func handleFSEvent(
	ctx context.Context,
	state *store.EngineState,
	event manifestFilesChangedAction) {
	manifest := state.ManifestStates[event.manifestName].Manifest

	if eventContainsConfigFiles(manifest, event) {
		logger.Get(ctx).Debugf("Event contains config files")
		state.ManifestStates[event.manifestName].ConfigIsDirty = true
	}

	ms := state.ManifestStates[event.manifestName]

	for _, f := range event.files {
		ms.FileChangesSinceLastBuild[f] = true
		ms.FileChangesSinceLastSuccessfulBuild[f] = true
	}

	spurious, err := onlySpuriousChanges(ms.FileChangesSinceLastBuild)
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
	if len(forwards) < len(ms.Manifest.PortForwards) {
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
	cStatus, err := k8s.ContainerMatching(pod, ms.Manifest.DockerRef)
	if err != nil {
		logger.Get(ctx).Debugf("Error matching container: %v", err)
		return
	} else if cStatus.Name == "" {
		return
	}
	populateContainerStatus(ctx, ms, pod, cStatus)

	if int(cStatus.RestartCount) > ms.Pod.ContainerRestarts {
		ms.Pod.PreRestartLog = append([]byte{}, ms.Pod.Log...)
		ms.Pod.Log = []byte{}
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

	ms.Pod.Log = append(ms.Pod.Log, action.Log...)
}

func handleServiceEvent(ctx context.Context, state *store.EngineState, service *v1.Service) {
	manifestName := model.ManifestName(service.ObjectMeta.Labels[ManifestNameLabel])
	if manifestName == "" {
		return
	}

	ms, ok := state.ManifestStates[manifestName]
	if !ok {
		logger.Get(ctx).Infof("error: got notified of service for unknown manifest '%s'", manifestName)
		return
	}

	url, err := k8s.ServiceURL(service)
	if err != nil {
		logger.Get(ctx).Infof("error resolving service %s: %v", manifestName, err)
		return
	}

	ms.LBs[k8s.ServiceName(service.Name)] = url
}

func (u Upper) handleInitAction(ctx context.Context, engineState *store.EngineState, action InitAction) error {
	watchMounts := action.WatchMounts
	manifests := action.Manifests

	for _, m := range manifests {
		engineState.ManifestDefinitionOrder = append(engineState.ManifestDefinitionOrder, m.Name)
		engineState.ManifestStates[m.Name] = store.NewManifestState(m)
	}
	engineState.WatchMounts = watchMounts

	var err error
	if watchMounts {
		// TODO(nick): The watchers should be in a subscriber.
		err = makeManifestWatcher(ctx, u.store, u.fsWatcherMaker, u.timerMaker, manifests)
		if err != nil {
			return err
		}
		err = u.podWatcherMaker(ctx, u.store)
		if err != nil {
			return err
		}
		err = u.serviceWatcherMaker(ctx, u.store)
		if err != nil {
			return err
		}

		go func() {
			err := u.reapOldWatchBuilds(ctx, manifests, time.Now())
			if err != nil {
				logger.Get(ctx).Debugf("Error garbage collecting builds: %v", err)
			}
		}()
	}

	for _, m := range manifests {
		enqueueBuild(engineState, m.Name)
	}
	engineState.InitialBuildCount = len(engineState.ManifestsToBuild)
	return nil
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

func eventContainsConfigFiles(manifest model.Manifest, e manifestFilesChangedAction) bool {
	matcher, err := manifest.ConfigMatcher()
	if err != nil {
		return false
	}

	for _, f := range e.files {
		matches, err := matcher.Matches(f, false)
		if matches && err == nil {
			return true
		}
	}

	return false
}

func getNewManifestFromTiltfile(name model.ManifestName) (model.Manifest, *manifestErr) {
	t, err := tiltfile.Load(tiltfile.FileName, os.Stdout)
	if err != nil {
		return model.Manifest{}, manifestErrf(err.Error())
	}
	newManifests, err := t.GetManifestConfigs(string(name))
	if err != nil {
		return model.Manifest{}, manifestErrf(err.Error())
	}
	if len(newManifests) != 1 {
		return model.Manifest{}, manifestErrf("Expected there to be 1 manifest for %s, got %d", name, len(newManifests))
	}
	newManifest := newManifests[0]

	return newManifest, nil
}

func (u Upper) resolveLB(ctx context.Context, spec k8s.LoadBalancerSpec) *url.URL {
	lb, _ := u.k8s.ResolveLoadBalancer(ctx, spec)
	return lb.URL
}

func (u Upper) reapOldWatchBuilds(ctx context.Context, manifests []model.Manifest, createdBefore time.Time) error {
	refs := make([]reference.Named, len(manifests))
	for i, s := range manifests {
		refs[i] = s.DockerRef
	}

	watchFilter := build.FilterByLabelValue(build.BuildMode, build.BuildModeExisting)
	for _, ref := range refs {
		nameFilter := build.FilterByRefName(ref)
		err := u.reaper.RemoveTiltImages(ctx, createdBefore, false, watchFilter, nameFilter)
		if err != nil {
			return fmt.Errorf("reapOldWatchBuilds: %v", err)
		}
	}

	return nil
}

// TODO(nick): This should be in the HUD
func showError(ctx context.Context, state *store.EngineState, resourceNumber int) {
	if resourceNumber > len(state.ManifestDefinitionOrder) {
		logger.Get(ctx).Infof("Resource %d does not exist, so no log to print", resourceNumber)
		return
	}

	mn := state.ManifestDefinitionOrder[resourceNumber-1]

	ms := state.ManifestStates[mn]

	if ms.LastBuildFinishTime.Equal(time.Time{}) {
		logger.Get(ctx).Infof("Resource %d has no previous build, so no log to print", resourceNumber)
		return
	}

	if ms.LastError != nil {
		logger.Get(ctx).Infof("Last %s build log:", mn)
		logger.Get(ctx).Infof("──────────────────────────────────────────────────────────")
		logger.Get(ctx).Infof("%s", ms.LastBuildLog.String())
		logger.Get(ctx).Infof("──────────────────────────────────────────────────────────")
	} else {
		logger.Get(ctx).Infof("%s pod log:", mn)
		logger.Get(ctx).Infof("──────────────────────────────────────────────────────────")

		// attempting to include at most one crash:
		// if the current pod has crashed, then just print the current pod
		// if the current pod is live, print the current pod plus the last pod
		var s string
		if ms.Pod.ContainerReady {
			s = string(ms.Pod.PreRestartLog) + string(ms.Pod.Log)
		} else {
			s = string(ms.Pod.Log)
		}
		logger.Get(ctx).Infof("%s", s)
		logger.Get(ctx).Infof("──────────────────────────────────────────────────────────")
	}
}

type manifestErr struct {
	s string
}

func (e *manifestErr) Error() string { return e.s }

var _ error = &manifestErr{}

func manifestErrf(format string, a ...interface{}) *manifestErr {
	return &manifestErr{s: fmt.Sprintf(format, a...)}
}
