package dcwatch

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/filteredwriter"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const disableDebounceDelay = 200 * time.Millisecond

type DisableSubscriber struct {
	dcc            dockercompose.DockerComposeClient
	clock          clockwork.Clock
	mu             sync.Mutex
	resourceStates map[string]resourceState

	// track the start times of containers we've already tried to rm, so we don't try to rm state we've already
	// processed
	// (the subscriber will continue reporting that the resource needs cleanup until we successfully kill the
	// container and the DC event watcher notices and updates EngineState)
	lastDisableStartTimes map[string]time.Time

	// since the goroutines are generally unobservable no-ops if we do something bad like spawn one for every OnChange,
	// we need to instrument for observability in testing
	goroutinesSpawnedForTesting int
}

type resourceState struct {
	Spec                model.DockerComposeUpSpec
	NeedsCleanup        bool
	CurrentlyCleaningUp bool
	// the container's start time
	StartTime time.Time
	// the service's order wrt other services, to ensure logging in consistent order
	Order int
}

func NewDisableSubscriber(dcc dockercompose.DockerComposeClient, clock clockwork.Clock) *DisableSubscriber {
	return &DisableSubscriber{
		dcc:                   dcc,
		clock:                 clock,
		resourceStates:        make(map[string]resourceState),
		lastDisableStartTimes: make(map[string]time.Time),
	}
}

func (w *DisableSubscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	state := st.RLockState()

	project := state.DockerComposeProject()
	if model.IsEmptyDockerComposeProject(project) {
		st.RUnlockState()
		return nil
	}

	var toCleanup []string

	for i, mn := range state.ManifestDefinitionOrder {
		name := mn.String()
		mt := state.ManifestTargets[mn]
		ms := mt.State

		if !ms.IsDC() {
			continue
		}

		runtimeStatus := ms.DCRuntimeState().RuntimeStatus()

		isRunning := runtimeStatus == v1alpha1.RuntimeStatusOK || runtimeStatus == v1alpha1.RuntimeStatusPending
		isDisabled := ms.DisableState == v1alpha1.DisableStateDisabled
		needsCleanup := isRunning && isDisabled

		rs := resourceState{
			Spec:         mt.Manifest.DockerComposeTarget().Spec,
			NeedsCleanup: needsCleanup,
			StartTime:    ms.DCRuntimeState().StartTime,
			Order:        i,
		}
		w.mu.Lock()
		rs.CurrentlyCleaningUp = w.resourceStates[name].CurrentlyCleaningUp
		if rs.NeedsCleanup && !rs.CurrentlyCleaningUp {
			rs.CurrentlyCleaningUp = true
			toCleanup = append(toCleanup, name)
		}
		w.resourceStates[name] = rs
		w.mu.Unlock()
	}

	st.RUnlockState()

	if len(toCleanup) > 0 {
		w.goroutinesSpawnedForTesting += 1

		go func() {

			// docker-compose rm can take 5-10 seconds
			// we sleep a bit here so that if a bunch of resources are disabled in bulk, we do them all at once rather
			// than starting the first one we see, and then getting the rest in a second docker-compose rm call
			select {
			case <-ctx.Done():
				return
			case <-w.clock.After(disableDebounceDelay):
			}

			w.Reconcile(ctx)
			w.mu.Lock()
			for _, name := range toCleanup {
				rs := w.resourceStates[name]
				rs.CurrentlyCleaningUp = false
				w.resourceStates[name] = rs
			}
			w.mu.Unlock()
		}()
	}

	return nil
}

func (w *DisableSubscriber) Reconcile(ctx context.Context) {
	var toDisable []model.DockerComposeUpSpec

	w.mu.Lock()

	for _, entry := range w.resourceStates {
		lastDisableStartTime := w.lastDisableStartTimes[entry.Spec.Service]
		if entry.NeedsCleanup && entry.StartTime.After(lastDisableStartTime) {
			toDisable = append(toDisable, entry.Spec)
			w.lastDisableStartTimes[entry.Spec.Service] = entry.StartTime
		}
	}

	sort.Slice(toDisable, func(i, j int) bool {
		return w.resourceStates[toDisable[i].Service].Order < w.resourceStates[toDisable[j].Service].Order
	})

	w.mu.Unlock()

	if len(toDisable) == 0 {
		return
	}

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

	out = filteredwriter.New(out, func(s string) bool {
		// https://app.shortcut.com/windmill/story/13147/docker-compose-down-messages-for-disabled-resources-may-be-confusing
		return strings.HasPrefix(s, "Going to remove")
	})

	err := w.dcc.Rm(ctx, toDisable, out, out)
	if err != nil {
		var namesToDisable []string
		for _, e := range toDisable {
			namesToDisable = append(namesToDisable, e.Service)
		}
		logger.Get(ctx).Errorf("error stopping disabled docker compose services %v, error: %v", namesToDisable, err)
	}
}
