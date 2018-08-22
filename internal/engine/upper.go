package engine

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"
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
}

type watcherMaker func() (watch.Notify, error)
type timerMaker func(d time.Duration) <-chan time.Time

func NewUpper(ctx context.Context, manager service.Manager, env k8s.Env) (Upper, error) {
	b, err := NewLocalBuildAndDeployer(ctx, manager, env)
	if err != nil {
		return Upper{}, err
	}
	watcherMaker := func() (watch.Notify, error) {
		return watch.NewWatcher()
	}
	return Upper{b, watcherMaker, time.After}, nil
}

func (u Upper) Up(ctx context.Context, services []model.Service, watchMounts bool) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-Up")
	defer span.Finish()

	buildTokens := make(map[model.ServiceName]*buildToken)

	var sw *serviceWatcher
	var err error
	if watchMounts {
		sw, err = makeServiceWatcher(ctx, u.watcherMaker, u.timerMaker, services)
		if err != nil {
			return err
		}
	}

	for _, service := range services {
		buildToken, err := u.b.BuildAndDeploy(ctx, service, nil, nil)
		buildTokens[service.Name] = buildToken
		if err != nil {
			return err
		}
	}
	logger.Get(ctx).Debugf("[timing.py] finished initial build") // hook for timing.py

	if watchMounts {
		for {
			select {
			case event := <-sw.events:
				var changedPathsToPrint []string
				if len(event.files) > maxChangedFilesToPrint {
					changedPathsToPrint = append(event.files[:maxChangedFilesToPrint], "...")
				} else {
					changedPathsToPrint = event.files
				}

				logger.Get(ctx).Infof("files changed. rebuilding %v. observed changes: %v", event.service.Name, changedPathsToPrint)

				var err error
				buildTokens[event.service.Name], err = u.b.BuildAndDeploy(
					ctx,
					event.service,
					buildTokens[event.service.Name],
					event.files)
				if err != nil {
					logger.Get(ctx).Infof("build failed: %v", err.Error())
				}
				logger.Get(ctx).Debugf("[timing.py] finished build from file change") // hook for timing.py

			case err := <-sw.errs:
				return err
			}
		}
	}
	return nil
}
