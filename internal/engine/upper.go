package engine

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	build "github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/hud"
	k8s "github.com/windmilleng/tilt/internal/k8s"
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

// TODO(nick): maybe this should be called 'BuildEngine' or something?
// Upper seems like a poor and undescriptive name.
type Upper struct {
	b            BuildAndDeployer
	watcherMaker watcherMaker
	timerMaker   timerMaker
	k8s          k8s.Client
	browserMode  BrowserMode
	reaper       build.ImageReaper
	hud          hud.HeadsUpDisplay
}

type watcherMaker func() (watch.Notify, error)
type timerMaker func(d time.Duration) <-chan time.Time

func NewUpper(ctx context.Context, b BuildAndDeployer, k8s k8s.Client, browserMode BrowserMode,
	reaper build.ImageReaper, hud hud.HeadsUpDisplay) Upper {
	watcherMaker := func() (watch.Notify, error) {
		return watch.NewWatcher()
	}

	// Run the HUD in the background
	go hud.Run(ctx)

	return Upper{
		b:            b,
		watcherMaker: watcherMaker,
		timerMaker:   time.After,
		k8s:          k8s,
		browserMode:  browserMode,
		reaper:       reaper,
		hud:          hud,
	}
}

func (u Upper) CreateManifests(ctx context.Context, manifests []model.Manifest, watchMounts bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-Up")
	defer span.Finish()

	engineState := newState()

	var sw *manifestWatcher
	var err error
	if watchMounts {
		sw, err = makeManifestWatcher(ctx, u.watcherMaker, u.timerMaker, manifests)
		if err != nil {
			return err
		}
	}

	s := summary.NewSummary()
	err = s.Gather(manifests)
	if err != nil {
		return err
	}

	lbs := make([]k8s.LoadBalancerSpec, 0)
	for _, manifest := range manifests {
		engineState.manifestStates[manifest.Name] = newManifestState(manifest)

		buildResult, err := u.b.BuildAndDeploy(ctx, manifest, BuildStateClean)
		if err == nil {
			engineState.manifestStates[manifest.Name].lastBuild = NewBuildState(buildResult)
			lbs = append(lbs, k8s.ToLoadBalancerSpecs(buildResult.Entities)...)
		} else if isPermanentError(err) {
			return err
		} else if watchMounts {
			o := output.Get(ctx)
			logger.Get(ctx).Infof("%s", o.Red().Sprintf("build failed: %v", err))
		} else {
			return fmt.Errorf("build failed: %v", err)
		}
	}

	if len(lbs) > 0 && u.browserMode == BrowserAuto {
		// Open only the first load balancer in a browser.
		// TODO(nick): We might need some hints on what load balancer to
		// open if we have multiple, or what path to default to on the opened manifest.
		err := k8s.OpenService(ctx, u.k8s, lbs[0])
		if err != nil {
			return err
		}
	}

	logger.Get(ctx).Debugf("[timing.py] finished initial build") // hook for timing.py

	s.Log(ctx, u.resolveLB)

	if watchMounts {
		go func() {
			err := u.reapOldWatchBuilds(ctx, manifests, time.Now())
			if err != nil {
				logger.Get(ctx).Debugf("Error garbage collecting builds: %v", err)
			}
		}()

		logger.Get(ctx).Infof("Awaiting edits...")

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case fsEvent := <-sw.events:
				u.handleFSEvent(ctx, engineState, fsEvent)
			case completedBuild := <-engineState.completedBuilds:
				err := u.handleCompletedBuild(ctx, completedBuild, engineState, s)
				if err != nil {
					return err
				}
			case err := <-sw.errs:
				return err
			}
			u.dispatch(ctx, engineState)
		}
	}
	return nil
}

func (u Upper) dispatch(ctx context.Context, state *engineState) {
	if len(state.manifestsToBuild) == 0 || state.currentlyBuilding != "" {
		return
	}

	mn := state.manifestsToBuild[0]
	state.manifestsToBuild = state.manifestsToBuild[1:]
	state.currentlyBuilding = mn
	ms := state.manifestStates[mn]

	var buildState BuildState
	if ms.configIsDirty {
		newManifest, err := getNewManifestFromTiltfile(ctx, mn)
		if err != nil {
			logger.Get(ctx).Infof("getting new manifest error: %v", err)
			state.currentlyBuilding = ""
			return
		}
		ms.lastBuild = BuildStateClean
		ms.manifest = newManifest
		ms.configIsDirty = false
		buildState = ms.lastBuild
	} else {
		for f := range ms.pendingFileChanges {
			ms.currentlyBuildingFileChanges = append(ms.currentlyBuildingFileChanges, f)
		}
		ms.pendingFileChanges = make(map[string]bool)

		buildState = ms.lastBuild.NewStateWithFilesChanged(ms.currentlyBuildingFileChanges)
	}

	m := ms.manifest

	go func() {
		u.logBuildEvent(ctx, m, buildState)

		result, err := u.b.BuildAndDeploy(
			ctx,
			m,
			buildState)

		state.completedBuilds <- completedBuild{result, err}
	}()
}

func (u Upper) handleCompletedBuild(ctx context.Context, cb completedBuild, engineState *engineState, s *summary.Summary) error {
	defer func() {
		engineState.currentlyBuilding = ""
	}()

	err := cb.err

	if err != nil {
		if isPermanentError(err) {
			return err
		}

		o := output.Get(ctx)
		logger.Get(ctx).Infof("%s", o.Red().Sprintf("build failed: %v", err))
	} else {
		ms := engineState.manifestStates[engineState.currentlyBuilding]

		ms.lastBuild = NewBuildState(cb.result)
		ms.currentlyBuildingFileChanges = nil
	}
	logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py

	s.Log(ctx, u.resolveLB)
	logger.Get(ctx).Infof("Awaiting changes…")

	return nil
}

func (u Upper) handleFSEvent(
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

	state.manifestsToBuild = append(state.manifestsToBuild, event.manifestName)
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

func (u Upper) logBuildEvent(ctx context.Context, manifest model.Manifest, buildState BuildState) {
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
