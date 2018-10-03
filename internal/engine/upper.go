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

func (u Upper) CreateManifests(ctx context.Context, manifests []model.Manifest, watchMounts bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-Up")
	defer span.Finish()

	buildStates := make(BuildStatesByName)

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
		buildStates[manifest.Name] = BuildStateClean

		buildResult, err := u.b.BuildAndDeploy(ctx, manifest, BuildStateClean)
		if err == nil {
			buildStates[manifest.Name] = NewBuildState(buildResult)
			lbs = append(lbs, k8s.ToLoadBalancerSpecs(buildResult.Entities)...)
		} else if isPermanentError(err) {
			return err
		} else if watchMounts {
			o := output.Get(ctx)
			o.PrintColorf(o.Red(), "build failed: %v", err)
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

	output.Get(ctx).Summary(s.Output(ctx, u.resolveLB))

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
			case event := <-sw.events:
				if eventContainsConfigFiles(event) {
					t, err := tiltfile.Load(tiltfile.FileName, os.Stdout)
					if err != nil {
						// TODO(dmiller) should we fail here, or is this OK?
						return err
					}
					newManifests, err := t.GetManifestConfigs(string(event.manifest.Name))
					if err != nil {
						return err
					}
					if len(newManifests) != 1 {
						return fmt.Errorf("Expected there to be 1 manifest for %s, got %d", event.manifest.Name, len(manifests))
					}
					newManifest := newManifests[0]
					buildState := BuildStateClean
					err = u.buildManifestFromBuildState(ctx, newManifest, buildState, buildStates)
					if err != nil {
						return err
					}
				} else {
					oldState := buildStates[event.manifest.Name]
					buildState := oldState.NewStateWithFilesChanged(event.files)
					buildStates[event.manifest.Name] = buildState

					spurious, err := buildState.OnlySpuriousChanges()
					if err != nil {
						logger.Get(ctx).Infof("build watch error: %v", err)
					}

					if spurious {
						// TODO(nick): I think we probably want to log when this happens?
						continue
					}

					err = u.buildManifestFromBuildState(ctx, event.manifest, buildState, buildStates)
					if err != nil {
						return err
					}

					output.Get(ctx).Summary(s.Output(ctx, u.resolveLB))
					output.Get(ctx).Printf("Awaiting changes…")
				}
			case err := <-sw.errs:
				return err
			}
		}
	}
	return nil
}

func eventContainsConfigFiles(e manifestFilesChangedEvent) bool {
	if e.manifest.ConfigMatcher == nil {
		return false
	}

	for _, f := range e.files {
		matches, err := e.manifest.ConfigMatcher.Matches(f, false)
		if matches && err == nil {
			return true
		}
	}

	return false
}

func (u Upper) buildManifestFromBuildState(ctx context.Context, m model.Manifest, b BuildState, buildStates BuildStatesByName) error {
	u.logBuildEvent(ctx, m, b)

	result, err := u.b.BuildAndDeploy(ctx, m, b)
	if err != nil {
		if isPermanentError(err) {
			return err
		}
		o := output.Get(ctx)
		o.PrintColorf(o.Red(), "build failed: %v", err)
	} else {
		buildStates[m.Name] = NewBuildState(result)
		logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py
	}
	return nil
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
