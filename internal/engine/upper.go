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
	"github.com/windmilleng/tilt/internal/output"
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
	b               BuildAndDeployer
	fsWatcherMaker  fsWatcherMaker
	timerMaker      timerMaker
	podWatcherMaker PodWatcherMaker
	k8s             k8s.Client
	browserMode     BrowserMode
	reaper          build.ImageReaper
	hud             hud.HeadsUpDisplay
}

type fsWatcherMaker func() (watch.Notify, error)
type PodWatcherMaker func(context.Context) (*podWatcher, error)
type timerMaker func(d time.Duration) <-chan time.Time

func ProvidePodWatcherMaker(kCli k8s.Client) PodWatcherMaker {
	return func(ctx context.Context) (*podWatcher, error) {
		return makePodWatcher(ctx, kCli)
	}
}

func NewUpper(ctx context.Context, b BuildAndDeployer, k8s k8s.Client, browserMode BrowserMode,
	reaper build.ImageReaper, hud hud.HeadsUpDisplay, pw PodWatcherMaker) Upper {
	fsWatcherMaker := func() (watch.Notify, error) {
		return watch.NewWatcher()
	}

	// Run the HUD in the background
	go func() {
		err := hud.Run(ctx)
		if err != nil {
			//TODO(matt) this might not be the best thing to do with an error - seems easy to miss
			logger.Get(ctx).Infof("error in hud: %v", err)
		}
	}()

	return Upper{
		b:               b,
		fsWatcherMaker:  fsWatcherMaker,
		podWatcherMaker: pw,
		timerMaker:      time.After,
		k8s:             k8s,
		browserMode:     browserMode,
		reaper:          reaper,
		hud:             hud,
	}
}

func (u Upper) CreateManifests(ctx context.Context, manifests []model.Manifest, watchMounts bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-Up")
	defer span.Finish()

	engineState := newState(manifests)

	var sw *manifestWatcher
	var pw *podWatcher
	var err error
	if watchMounts {
		sw, err = makeManifestWatcher(ctx, u.fsWatcherMaker, u.timerMaker, manifests)
		if err != nil {
			return err
		}
		pw, err = u.podWatcherMaker(ctx)
		if err != nil {
			return err
		}

		go func() {
			err := u.reapOldWatchBuilds(ctx, manifests, time.Now())
			if err != nil {
				logger.Get(ctx).Debugf("Error garbage collecting builds: %v", err)
			}
		}()
	} else {
		sw = newDummyManifestWatcher()
		pw = newDummyPodWatcher()
	}

	s := summary.NewSummary()
	err = s.Gather(manifests)
	if err != nil {
		return err
	}

	for _, m := range manifests {
		enqueueBuild(engineState, m.Name)
	}
	engineState.initialBuildCount = len(engineState.manifestsToBuild)

	if u.browserMode == BrowserAuto {
		engineState.openBrowserOnNextLB = true
	}

	for {
		u.dispatch(ctx, engineState)
		err := u.hud.Update(stateToView(*engineState))
		if err != nil {
			logger.Get(ctx).Infof("Error updating HUD: %v", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case fsEvent := <-sw.events:
			handleFSEvent(ctx, engineState, fsEvent)
		case completedBuild := <-engineState.completedBuilds:
			err := u.handleCompletedBuild(ctx, watchMounts, completedBuild, engineState, s)
			if err != nil {
				return err
			}
			if !watchMounts && len(engineState.manifestsToBuild) == 0 {
				return nil
			}
		case pod := <-pw.events:
			handlePodEvent(ctx, engineState, pod)
		case err := <-sw.errs:
			return err
		case <-time.After(refreshInterval):
			break
		}
	}
}

func (u Upper) dispatch(ctx context.Context, state *engineState) {
	if len(state.manifestsToBuild) == 0 || state.currentlyBuilding != "" {
		return
	}

	mn := state.manifestsToBuild[0]
	state.manifestsToBuild = state.manifestsToBuild[1:]
	state.currentlyBuilding = mn
	ms := state.manifestStates[mn]
	ms.queueEntryTime = time.Time{}

	if ms.configIsDirty {
		newManifest, err := getNewManifestFromTiltfile(ctx, mn)
		if err != nil {
			logger.Get(ctx).Infof("getting new manifest error: %v", err)
			state.currentlyBuilding = ""
			ms.lastError = err
			ms.lastBuildFinishTime = time.Now()
			ms.lastBuildDuration = 0
			return
		}
		ms.lastBuild = BuildStateClean
		ms.manifest = newManifest
		ms.configIsDirty = false
	}

	for f := range ms.pendingFileChanges {
		ms.currentlyBuildingFileChanges = append(ms.currentlyBuildingFileChanges, f)
	}
	ms.pendingFileChanges = make(map[string]bool)

	buildState := ms.lastBuild.NewStateWithFilesChanged(ms.currentlyBuildingFileChanges)

	m := ms.manifest

	ms.currentBuildStartTime = time.Now()

	ctx = output.CtxWithForkedOutput(ctx, &ms.currentBuildLog)

	go func() {
		firstBuild := !ms.hasBeenBuilt
		ms.hasBeenBuilt = true

		u.logBuildEvent(ctx, firstBuild, m, buildState)

		result, err := u.b.BuildAndDeploy(
			ctx,
			m,
			buildState)

		state.completedBuilds <- completedBuild{result, err}
	}()
}

func (u Upper) handleCompletedBuild(ctx context.Context, watching bool, cb completedBuild, engineState *engineState, s *summary.Summary) error {
	defer func() {
		engineState.currentlyBuilding = ""
	}()

	engineState.completedBuildCount++

	defer func() {
		if engineState.completedBuildCount == engineState.initialBuildCount {
			logger.Get(ctx).Debugf("[timing.py] finished initial build") // hook for timing.py
		}
	}()

	err := cb.err

	ms := engineState.manifestStates[engineState.currentlyBuilding]
	ms.lastError = err
	ms.lastBuildFinishTime = time.Now()
	ms.lastBuildDuration = time.Since(ms.currentBuildStartTime)
	ms.currentBuildStartTime = time.Time{}
	ms.lastBuildLog = ms.currentBuildLog
	ms.currentBuildLog = bytes.Buffer{}

	if err != nil {
		if isPermanentError(err) {
			return err
		} else if watching {
			o := output.Get(ctx)
			logger.Get(ctx).Infof("%s", o.Red().Sprintf("build failed: %v", err))
		} else {
			return fmt.Errorf("build failed: %v", err)
		}
	} else {
		ms.lastSuccessfulDeployTime = time.Now()

		ms.lbs = k8s.ToLoadBalancerSpecs(cb.result.Entities)

		if len(ms.lbs) > 0 && engineState.openBrowserOnNextLB {
			// Open only the first load balancer in a browser.
			// TODO(nick): We might need some hints on what load balancer to
			// open if we have multiple, or what path to default to on the opened manifest.
			err := k8s.OpenService(ctx, u.k8s, ms.lbs[0])
			if err != nil {
				return err
			}
			engineState.openBrowserOnNextLB = false
		}

		ms.lastBuild = NewBuildState(cb.result)
		ms.lastSuccessfulDeployEdits = ms.currentlyBuildingFileChanges
		ms.currentlyBuildingFileChanges = nil
	}

	if watching {
		logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py

		s.Log(ctx, u.resolveLB)
		if len(engineState.manifestsToBuild) == 0 {
			logger.Get(ctx).Infof("Awaiting changes…")
		}
	}

	return nil
}

func handleFSEvent(
	ctx context.Context,
	state *engineState,
	event manifestFilesChangedEvent) {

	manifest := state.manifestStates[event.manifestName].manifest

	if eventContainsConfigFiles(manifest, event) {
		logger.Get(ctx).Debugf("Event contains config files")
		state.manifestStates[event.manifestName].configIsDirty = true
	}

	ms := state.manifestStates[event.manifestName]

	for _, f := range event.files {
		ms.pendingFileChanges[f] = true
	}

	spurious, err := onlySpuriousChanges(ms.pendingFileChanges)
	if err != nil {
		logger.Get(ctx).Infof("build watch error: %v", err)
	}

	if spurious {
		// TODO(nick): I think we probably want to log when this happens?
		return
	}

	// if the name is already in the queue, we don't need to add it again
	for _, mn := range state.manifestsToBuild {
		if mn == event.manifestName {
			return
		}
	}

	enqueueBuild(state, event.manifestName)
}

func enqueueBuild(state *engineState, mn model.ManifestName) {
	state.manifestsToBuild = append(state.manifestsToBuild, mn)
	state.manifestStates[mn].queueEntryTime = time.Now()
}

func handlePodEvent(ctx context.Context, state *engineState, pod *v1.Pod) {
	manifestName := model.ManifestName(pod.ObjectMeta.Labels[ManifestNameLabel])
	if manifestName == "" {
		return
	}

	newPod := Pod{
		Name:      pod.Name,
		StartedAt: pod.CreationTimestamp.Time,
		Status:    podStatusToString(*pod),
	}

	ms, ok := state.manifestStates[manifestName]
	if !ok {
		logger.Get(ctx).Infof("error: got notified of pod for unknown manifest '%s'", manifestName)
		return
	}

	oldPod := ms.pod

	if oldPod == unknownPod || oldPod.Name == newPod.Name || oldPod.StartedAt.Before(newPod.StartedAt) {
		ms.pod = newPod
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

func eventContainsConfigFiles(manifest model.Manifest, e manifestFilesChangedEvent) bool {
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

func (u Upper) logBuildEvent(ctx context.Context, firstBuild bool, manifest model.Manifest, buildState BuildState) {
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

var _ model.ManifestCreator = Upper{}
