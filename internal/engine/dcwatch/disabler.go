package dcwatch

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const disableDebounceDelay = 200 * time.Millisecond

// disables DC resources w/ debouncing to group them into one docker-compose invocation
type disabler struct {
	dcc      dockercompose.DockerComposeClient
	clock    clockwork.Clock
	updates  chan disableQueueEntry
	loopOnce sync.Once
}

func newDisabler(dcc dockercompose.DockerComposeClient, clock clockwork.Clock) disabler {
	return disabler{
		dcc:     dcc,
		clock:   clock,
		updates: make(chan disableQueueEntry),
	}
}

type disableQueueEntry struct {
	Spec         model.DockerComposeUpSpec
	NeedsCleanup bool
	// the container's start time
	StartTime time.Time
	// the service's order wrt other services, to ensure logging in consistent order
	Order int
}

func (d *disabler) Update(ctx context.Context, e disableQueueEntry) {
	d.loopOnce.Do(func() {
		go d.Loop(ctx)
	})

	d.updates <- e
}

func (d *disabler) Loop(ctx context.Context) {
	// track the start times of containers we've already tried to rm, so we don't try to rm state we've already
	// processed
	// (the subscriber will continue reporting that the resource needs cleanup until we successfully kill the
	// container and the DC event watcher notices and updates EngineState)
	lastDisableStartTimes := make(map[string]time.Time)

	for {
		entries := make(map[string]disableQueueEntry)
		select {
		case entry := <-d.updates:
			entries[entry.Spec.Service] = entry
		case <-ctx.Done():
			return
		}

		// docker-compose rm can take 5-10 seconds
		// we sleep a bit here so that if a bunch of resources are disabled in bulk, we do them all at once rather
		// than starting the first one we see, and then getting the rest in a second docker-compose rm call
		debounce := d.clock.After(disableDebounceDelay)
		done := false
		for !done {
			select {
			case entry := <-d.updates:
				entries[entry.Spec.Service] = entry
			case <-debounce:
				done = true
			case <-ctx.Done():
				return
			}
		}

		var toDisable []model.DockerComposeUpSpec

		for _, entry := range entries {
			lastDisableStartTime := lastDisableStartTimes[entry.Spec.Service]
			if entry.NeedsCleanup && entry.StartTime.After(lastDisableStartTime) {
				toDisable = append(toDisable, entry.Spec)
				lastDisableStartTimes[entry.Spec.Service] = entry.StartTime
			}
		}

		if len(toDisable) == 0 {
			continue
		}

		sort.Slice(toDisable, func(i, j int) bool {
			return entries[toDisable[i].Service].Order < entries[toDisable[j].Service].Order
		})

		// Upon disabling, the DC event watcher will notice the container has stopped and update
		// the resource's RuntimeStatus, preventing it from being re-added to specsToDisable.

		// NB: For now, DC output only goes to the global log
		// 1. `docker-compose` rm is slow, so we don't want to call it serially, once per resource
		// 2. we've had bad luck with concurrent `docker-compose` processes, so we don't want to do it in parallel
		// 3. we can't break the DC output up by resource
		// 4. our logger doesn't support writing the same span to multiple manifests
		//    (https://app.shortcut.com/windmill/story/13140/support-logging-to-multiple-manifests)

		// `docker-compose rm` output is a bit of a pickle. On one hand, the command can take several seconds,
		// so it's nice to let it write to the log in real time (rather than only on error), to give the user
		// feedback that something is happening. On the other hand, `docker-compose rm` does tty tricks that
		// don't work in the Tilt log, which makes it ugly.
		out := logger.Get(ctx).Writer(logger.InfoLvl)

		err := d.dcc.Rm(ctx, toDisable, out, out)
		if err != nil {
			var namesToDisable []string
			for _, e := range toDisable {
				namesToDisable = append(namesToDisable, e.Service)
			}
			logger.Get(ctx).Errorf("error stopping disabled docker compose services %v, error: %v", namesToDisable, err)
		}
	}
}
