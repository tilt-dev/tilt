package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	build "github.com/windmilleng/tilt/internal/build"
	k8s "github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/summary"
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
}

type watcherMaker func() (watch.Notify, error)
type timerMaker func(d time.Duration) <-chan time.Time

func NewUpper(ctx context.Context, b BuildAndDeployer, k8s k8s.Client, browserMode BrowserMode, reaper build.ImageReaper) Upper {
	watcherMaker := func() (watch.Notify, error) {
		return watch.NewWatcher()
	}
	return Upper{
		b:            b,
		watcherMaker: watcherMaker,
		timerMaker:   time.After,
		k8s:          k8s,
		browserMode:  browserMode,
		reaper:       reaper,
	}
}

func (u Upper) CreateServices(ctx context.Context, services []model.Service, watchMounts bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-Up")
	defer span.Finish()

	buildStates := make(map[model.ServiceName]BuildState)

	var sw *serviceWatcher
	var err error
	if watchMounts {
		sw, err = makeServiceWatcher(ctx, u.watcherMaker, u.timerMaker, services)
		if err != nil {
			return err
		}
	}

	s := summary.NewSummary()
	s.Gather(services)

	lbs := make([]k8s.LoadBalancer, 0)
	for _, service := range services {
		buildStates[service.Name] = BuildStateClean

		buildResult, err := u.b.BuildAndDeploy(ctx, service, BuildStateClean)
		if err == nil {
			buildStates[service.Name] = NewBuildState(buildResult)
			lbs = append(lbs, k8s.ToLoadBalancers(buildResult.Entities)...)
		} else if watchMounts {
			logger.Get(ctx).Infof("build failed: %v", err)
		} else {
			return fmt.Errorf("build failed: %v", err)
		}
	}

	if len(lbs) > 0 && u.browserMode == BrowserAuto {
		// Open only the first load balancer in a browser.
		// TODO(nick): We might need some hints on what load balancer to
		// open if we have multiple, or what path to default to on the opened service.
		err := u.k8s.OpenService(ctx, lbs[0])
		if err != nil {
			return err
		}
	}

	logger.Get(ctx).Debugf("[timing.py] finished initial build") // hook for timing.py

	output.Get(ctx).Printf("%s", s.Output())

	if watchMounts {
		go func() {
			err := u.reapOldWatchBuilds(ctx, services, time.Now())
			if err != nil {
				logger.Get(ctx).Infof("Error garbage collecting builds: %v", err)
			}
		}()

		// Give the pod(s) we just deployed a bit to come up
		time.Sleep(2 * time.Second)

		// Need to know what container(s) we just pushed up so we can do updates live.
		u.populateContainersForBuildStates(ctx, buildStates)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case event := <-sw.events:
				oldState := buildStates[event.service.Name]
				buildState := oldState.NewStateWithFilesChanged(event.files)
				buildStates[event.service.Name] = buildState

				spurious, err := buildState.OnlySpuriousChanges()
				if err != nil {
					logger.Get(ctx).Infof("build watch error: %v", err)
				}

				if spurious {
					// TODO(nick): I think we probably want to log when this happens?
					continue
				}

				u.logBuildEvent(ctx, event.service, buildState)

				result, err := u.b.BuildAndDeploy(
					ctx,
					event.service,
					buildState)
				if err != nil {
					logger.Get(ctx).Infof("build failed: %v", err)
				} else {
					buildStates[event.service.Name] = NewBuildState(result)
				}
				logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py

				output.Get(ctx).Printf("%s", s.Output())

			case err := <-sw.errs:
				return err
			}
		}
	}
	return nil
}

// populateContainersForBuildStates updates the given map of service --> build state in place;
// populates each BuildState with its corresponding containerID
func (u Upper) populateContainersForBuildStates(ctx context.Context, buildStates map[model.ServiceName]BuildState) {
	for serv, state := range buildStates {
		if !state.LastResult.HasContainer() && state.LastResult.HasImage() {
			cID, err := u.b.GetContainerForBuild(ctx, state.LastResult)
			if err != nil {
				logger.Get(ctx).Infof(
					"couldn't get container for %s, but the pod's probably just not up yet: %v", serv, err)
				continue
			}
			state.LastResult.Container = cID
			buildStates[serv] = state
		}
	}
}

func (u Upper) logBuildEvent(ctx context.Context, service model.Service, buildState BuildState) {
	changedFiles := buildState.FilesChanged()
	var changedPathsToPrint []string
	if len(changedFiles) > maxChangedFilesToPrint {
		changedPathsToPrint = append(changedPathsToPrint, changedFiles[:maxChangedFilesToPrint]...)
		changedPathsToPrint = append(changedPathsToPrint, "...")
	} else {
		changedPathsToPrint = changedFiles
	}

	logger.Get(ctx).Infof("  → %d changed: %v\n", len(changedFiles), ospath.TryAsCwdChildren(changedPathsToPrint))
	logger.Get(ctx).Infof("Rebuilding service: %s", service.Name)
}

func (u Upper) reapOldWatchBuilds(ctx context.Context, services []model.Service, createdBefore time.Time) error {
	refs := make([]reference.Named, len(services))
	for i, s := range services {
		refs[i] = s.DockerfileTag
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

var _ model.ServiceCreator = Upper{}
