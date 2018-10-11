package engine

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/pkg/browser"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/summary"
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

// The main loop ensures the HUD updates at least this often
const refreshInterval = 1 * time.Second

// TODO(nick): maybe this should be called 'BuildEngine' or something?
// Upper seems like a poor and undescriptive name.
type Upper struct {
	b                   BuildAndDeployer
	fsWatcherMaker      fsWatcherMaker
	timerMaker          timerMaker
	podWatcherMaker     PodWatcherMaker
	serviceWatcherMaker ServiceWatcherMaker
	k8s                 k8s.Client
	browserMode         BrowserMode
	reaper              build.ImageReaper
	hud                 hud.HeadsUpDisplay
	store               *store.Store
	plm                 *PodLogManager
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

func NewUpper(ctx context.Context, b BuildAndDeployer, k8s k8s.Client, browserMode BrowserMode,
	reaper build.ImageReaper, hud hud.HeadsUpDisplay, pwm PodWatcherMaker, swm ServiceWatcherMaker, st *store.Store, plm *PodLogManager) Upper {
	fsWatcherMaker := func() (watch.Notify, error) {
		return watch.NewWatcher()
	}

	return Upper{
		b:                   b,
		fsWatcherMaker:      fsWatcherMaker,
		podWatcherMaker:     pwm,
		serviceWatcherMaker: swm,
		timerMaker:          time.After,
		k8s:                 k8s,
		browserMode:         browserMode,
		reaper:              reaper,
		hud:                 hud,
		store:               st,
		plm:                 plm,
	}
}

func (u Upper) CreateManifests(ctx context.Context, manifests []model.Manifest, watchMounts bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-Up")
	defer span.Finish()

	u.store.Dispatch(InitAction{
		WatchMounts: watchMounts,
		Manifests:   manifests,
	})

	// Run the HUD in the background
	go func() {
		err := u.hud.Run(ctx, u.store)
		if err != nil {
			//TODO(matt) this might not be the best thing to do with an error - seems easy to miss
			logger.Get(ctx).Infof("error in hud: %v", err)
		}
	}()

	for {
		// Subscribers
		done, err := maybeFinished(u.store)
		if done {
			return err
		}
		u.maybeStartBuild(ctx, u.store)
		u.maybeUpdateHUD(ctx, u.store)

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
			u.store.UnlockMutableState()
		case <-time.After(refreshInterval):
			break
		}
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
		return u.handleCompletedBuild(ctx, state, action)
	case hud.ReplayBuildLogAction:
		replayBuildLog(ctx, state, action.ResourceNumber)
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

func (u Upper) maybeUpdateHUD(ctx context.Context, st *store.Store) {
	state := st.RLockState()
	if len(state.ManifestStates) == 0 {
		st.RUnlockState()
		return
	}

	view := store.StateToView(state)
	st.RUnlockState()

	err := u.hud.Update(view)
	if err != nil {
		logger.Get(ctx).Infof("Error updating HUD: %v", err)
	}
}

// TODO(nick): This should be broken up into a separate subscriber (that kicks
// off the build) and a reducer (that modifies state).
func (u Upper) maybeStartBuild(ctx context.Context, st *store.Store) {
	state := st.LockMutableState()
	defer st.UnlockMutableState()

	if len(state.ManifestStates) == 0 ||
		len(state.ManifestsToBuild) == 0 ||
		state.CurrentlyBuilding != "" {
		return
	}

	mn := state.ManifestsToBuild[0]
	state.ManifestsToBuild = state.ManifestsToBuild[1:]
	state.CurrentlyBuilding = mn
	ms := state.ManifestStates[mn]
	ms.QueueEntryTime = time.Time{}

	if ms.ConfigIsDirty {
		newManifest, err := getNewManifestFromTiltfile(ctx, mn)
		if err != nil {
			logger.Get(ctx).Infof("getting new manifest error: %v", err)
			state.CurrentlyBuilding = ""
			ms.LastError = err
			ms.LastBuildFinishTime = time.Now()
			ms.LastBuildDuration = 0
			return
		}
		ms.LastBuild = store.BuildStateClean
		ms.Manifest = newManifest
		ms.ConfigIsDirty = false
	}

	for f := range ms.PendingFileChanges {
		ms.CurrentlyBuildingFileChanges = append(ms.CurrentlyBuildingFileChanges, f)
	}
	ms.PendingFileChanges = make(map[string]bool)

	buildState := ms.LastBuild.NewStateWithFilesChanged(ms.CurrentlyBuildingFileChanges)

	m := ms.Manifest

	ms.CurrentBuildStartTime = time.Now()

	ctx = output.CtxWithForkedOutput(ctx, ms.CurrentBuildLog)

	go func() {
		firstBuild := !ms.HasBeenBuilt
		ms.HasBeenBuilt = true
		u.logBuildEvent(ctx, firstBuild, m, buildState)

		result, err := u.b.BuildAndDeploy(
			ctx,
			m,
			buildState)

		st.Dispatch(NewBuildCompleteAction(result, err))
	}()
}

func (u Upper) handleCompletedBuild(ctx context.Context, engineState *store.EngineState, cb BuildCompleteAction) error {
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
	ms.LastError = err
	ms.LastBuildFinishTime = time.Now()
	ms.LastBuildDuration = time.Since(ms.CurrentBuildStartTime)
	ms.CurrentBuildStartTime = time.Time{}
	ms.LastBuildLog = ms.CurrentBuildLog
	ms.CurrentBuildLog = &bytes.Buffer{}

	if err != nil {
		if isPermanentError(err) {
			return err
		} else if engineState.WatchMounts {
			o := output.Get(ctx)
			logger.Get(ctx).Infof("%s", o.Red().Sprintf("build failed: %v", err))
		} else {
			return fmt.Errorf("build failed: %v", err)
		}
	} else {
		ms.LastSuccessfulDeployTime = time.Now()

		prevBuild := ms.LastBuild
		ms.LastBuild = store.NewBuildState(cb.Result)
		ms.LastSuccessfulDeployEdits = ms.CurrentlyBuildingFileChanges
		ms.CurrentlyBuildingFileChanges = nil

		if engineState.WatchMounts {
			u.plm.PostProcessBuild(ctx, ms.Manifest.Name, cb.Result, prevBuild.LastResult)
		}
	}

	if engineState.WatchMounts {
		logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py

		summary := summary.NewSummary()
		err := summary.Gather(engineState.Manifests())
		if err != nil {
			// If the user edited their k8s YAML and it's currently malformed,
			// summary.Gather() might fail. This is OK. Just don't print the log right now.
			// A better reactive model might have a way to only collect the manifests
			// that are actively deployed.
			logger.Get(ctx).Debugf("handleCompletedBuild: %v", err)
		} else {
			summary.Log(ctx, u.resolveLB)
		}

		if len(engineState.ManifestsToBuild) == 0 {
			logger.Get(ctx).Infof("Awaiting changes…")
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

func enqueueBuild(state *store.EngineState, mn model.ManifestName) {
	state.ManifestsToBuild = append(state.ManifestsToBuild, mn)
	state.ManifestStates[mn].QueueEntryTime = time.Now()
}

func handlePodEvent(ctx context.Context, state *store.EngineState, pod *v1.Pod) {
	manifestName := model.ManifestName(pod.ObjectMeta.Labels[ManifestNameLabel])
	if manifestName == "" {
		return
	}

	podID := k8s.PodIDFromPod(pod)
	startedAt := pod.CreationTimestamp.Time
	status := podStatusToString(*pod)

	ms, ok := state.ManifestStates[manifestName]
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	oldPod := ms.Pod
	if oldPod.PodID == "" || oldPod.StartedAt.Before(startedAt) {
		ms.Pod = store.Pod{
			PodID:     podID,
			StartedAt: startedAt,
			Status:    status,
		}
	} else if oldPod.PodID == podID {
		ms.Pod.Status = status
		ms.Pod.StartedAt = startedAt
	}
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

	if url != nil && state.OpenBrowserOnNextLB {
		// Open only the first load balancer in a browser.
		// TODO(nick): We might need some hints on what load balancer to
		// open if we have multiple, or what path to default to on the opened manifest.
		err := browser.OpenURL(url.String())
		if err != nil {
			logger.Get(ctx).Infof("error opening service %s at %s: %v", service.Name, url.String(), err)
			return
		}

		state.OpenBrowserOnNextLB = false
	}
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

	if u.browserMode == BrowserAuto {
		engineState.OpenBrowserOnNextLB = true
	}
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

func getNewManifestFromTiltfile(ctx context.Context, name model.ManifestName) (model.Manifest, error) {
	t, err := tiltfile.Load(tiltfile.FileName, os.Stdout)
	if err != nil {
		return model.Manifest{}, err
	}
	newManifests, err := t.GetManifestConfigs(string(name))
	if err != nil {
		return model.Manifest{}, err
	}
	if len(newManifests) != 1 {
		return model.Manifest{}, fmt.Errorf("Expected there to be 1 manifest for %s, got %d", name, len(newManifests))
	}
	newManifest := newManifests[0]

	return newManifest, nil
}

func (u Upper) resolveLB(ctx context.Context, spec k8s.LoadBalancerSpec) *url.URL {
	lb, _ := u.k8s.ResolveLoadBalancer(ctx, spec)
	return lb.URL
}

func (u Upper) logBuildEvent(ctx context.Context, firstBuild bool, manifest model.Manifest, buildState store.BuildState) {
	if firstBuild {
		logger.Get(ctx).Infof("Building manifest: %s", manifest.Name)
	} else {
		changedFiles := buildState.FilesChanged()
		var changedPathsToPrint []string
		if len(changedFiles) > maxChangedFilesToPrint {
			changedPathsToPrint = append(changedPathsToPrint, changedFiles[:maxChangedFilesToPrint]...)
			changedPathsToPrint = append(changedPathsToPrint, "...")
		} else {
			changedPathsToPrint = changedFiles
		}

		logger.Get(ctx).Infof("  → %d changed: %v\n", len(changedFiles), ospath.TryAsCwdChildren(changedPathsToPrint))
		logger.Get(ctx).Infof("Rebuilding manifest: %s", manifest.Name)
	}
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
func replayBuildLog(ctx context.Context, state *store.EngineState, resourceNumber int) {
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

	logger.Get(ctx).Infof("Reprinting last build log for %s:", mn)
	logger.Get(ctx).Infof("%s", ms.LastBuildLog.String())
}

var _ model.ManifestCreator = Upper{}
