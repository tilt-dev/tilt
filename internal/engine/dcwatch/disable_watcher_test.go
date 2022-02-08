package dcwatch

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestDockerComposeDebounce(t *testing.T) {
	f := newDWFixture(t)
	f.createResource("m1", v1alpha1.DisableStateDisabled, "running")
	f.createResource("m2", v1alpha1.DisableStateEnabled, "running")
	f.onChange()
	require.Len(t, f.dcClient.RmCalls(), 0)

	f.setDisableState("m2", v1alpha1.DisableStateDisabled)

	f.onChange()

	f.clock.BlockUntil(2)
	f.clock.Advance(20 * disableDebounceDelay)

	call := f.rmCall(1)

	require.Equal(t, []string{"m1", "m2"}, stoppedServices(call))
}

func TestDockerComposeDontRetryOnSameStartTime(t *testing.T) {
	f := newDWFixture(t)
	f.createResource("m1", v1alpha1.DisableStateDisabled, "running")
	f.createResource("m2", v1alpha1.DisableStateEnabled, "running")
	f.onChange()
	require.Len(t, f.dcClient.RmCalls(), 0)

	f.setDisableState("m2", v1alpha1.DisableStateDisabled)

	f.onChange()

	f.clock.BlockUntil(2)
	f.clock.Advance(2 * disableDebounceDelay)

	call := f.rmCall(1)
	require.Equal(t, []string{"m1", "m2"}, stoppedServices(call))

	f.onChange()

	f.clock.BlockUntil(1)
	f.clock.Advance(2 * disableDebounceDelay)

	require.Neverf(t, func() bool {
		return len(f.dcClient.RmCalls()) > 1
	}, 20*time.Millisecond, time.Millisecond, "docker-compose should not be called again")
}

func TestDockerComposeRetryIfStartTimeChanges(t *testing.T) {
	f := newDWFixture(t)
	f.createResource("m1", v1alpha1.DisableStateDisabled, "running")
	f.createResource("m2", v1alpha1.DisableStateEnabled, "running")
	f.onChange()
	require.Len(t, f.dcClient.RmCalls(), 0)

	f.setDisableState("m2", v1alpha1.DisableStateDisabled)
	f.onChange()

	f.clock.BlockUntil(2)
	f.clock.Advance(2 * disableDebounceDelay)

	require.Eventually(t, func() bool {
		return len(f.dcClient.RmCalls()) == 1
	}, time.Second, 10*time.Millisecond, "docker-compose rm called")

	call := f.rmCall(1)
	require.Equal(t, []string{"m1", "m2"}, stoppedServices(call))

	// simulate restarting m2 by bumping its start time
	f.st.WithManifestState("m2", func(ms *store.ManifestState) {
		rs := ms.DCRuntimeState()
		rs.StartTime = f.clock.Now()
		ms.RuntimeState = rs
	})

	f.onChange()
	f.clock.BlockUntil(1)
	f.clock.Advance(2 * disableDebounceDelay)

	call = f.rmCall(2)
	require.Equal(t, []string{"m2"}, stoppedServices(call))
}

func TestDockerComposeDontDisableIfReenabledDuringDebounce(t *testing.T) {
	f := newDWFixture(t)
	f.createResource("m1", v1alpha1.DisableStateDisabled, "running")
	f.createResource("m2", v1alpha1.DisableStateDisabled, "running")

	f.onChange()

	f.clock.BlockUntil(1)

	// reenable m2 during debounce
	f.setDisableState("m2", v1alpha1.DisableStateEnabled)

	f.onChange()

	f.clock.Advance(2 * disableDebounceDelay)

	call := f.rmCall(1)

	require.Equal(t, []string{"m1"}, stoppedServices(call))
}

func TestDisableError(t *testing.T) {
	f := newDWFixture(t)

	f.createResource("m1", v1alpha1.DisableStateDisabled, "running")
	f.dcClient.RmError = errors.New("fake dc error")
	f.onChange()

	f.clock.BlockUntil(1)
	f.clock.Advance(2 * disableDebounceDelay)

	require.Eventually(t, func() bool {
		return strings.Contains(f.log.String(), "fake dc error")
	}, 20*time.Millisecond, time.Millisecond)
}

// Iterations of this subscriber have spawned goroutines for every onChange call, so try to
// verify it's not doing that.
func TestDontSpawnRedundantGoroutines(t *testing.T) {
	f := newDWFixture(t)
	f.createResource("m1", v1alpha1.DisableStateDisabled, "running")
	f.createResource("m2", v1alpha1.DisableStateDisabled, "running")

	for i := 0; i < 10; i++ {
		f.onChange()
	}

	if !assert.Never(t, func() bool {
		f.watcher.mu.Lock()
		defer f.watcher.mu.Unlock()
		return f.watcher.goroutinesSpawnedForTesting > 1
	}, 20*time.Millisecond, 1*time.Millisecond) {
		f.watcher.mu.Lock()
		defer f.watcher.mu.Unlock()
		require.Equal(t, 1, f.watcher.goroutinesSpawnedForTesting, "goroutines spawned")
	}

	f.clock.BlockUntil(1)
	f.clock.Advance(20 * disableDebounceDelay)

	call := f.rmCall(1)

	require.Equal(t, []string{"m1", "m2"}, stoppedServices(call))
}

type dwFixture struct {
	*tempdir.TempDirFixture
	t        *testing.T
	ctx      context.Context
	dcClient *dockercompose.FakeDCClient
	watcher  *DisableSubscriber
	clock    clockwork.FakeClock
	st       *store.TestingStore
	log      *bufsync.ThreadSafeBuffer
}

func newDWFixture(t *testing.T) *dwFixture {
	log := bufsync.NewThreadSafeBuffer()
	out := io.MultiWriter(log, os.Stdout)
	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(out))
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	dcClient := dockercompose.NewFakeDockerComposeClient(t, ctx)
	clock := clockwork.NewFakeClock()
	watcher := NewDisableSubscriber(dcClient, clock)
	st := store.NewTestingStore()
	return &dwFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
		dcClient:       dcClient,
		watcher:        watcher,
		clock:          clock,
		st:             st,
		log:            log,
	}
}

func (f *dwFixture) onChange() {
	err := f.watcher.OnChange(f.ctx, f.st, store.ChangeSummary{})
	require.NoError(f.t, err)
}

func (f *dwFixture) createResource(mn model.ManifestName, disableState v1alpha1.DisableState, containerStatus string) {
	m := manifestbuilder.New(f, mn).WithDockerCompose().Build()
	mt := store.NewManifestTarget(m)
	mt.State.DisableState = disableState
	mt.State.RuntimeState = dockercompose.State{
		ContainerState: types.ContainerState{Status: containerStatus},
		StartTime:      f.clock.Now(),
	}

	f.st.WithState(func(state *store.EngineState) {
		state.UpsertManifestTarget(mt)
	})
}

func (f *dwFixture) setDisableState(mn model.ManifestName, ds v1alpha1.DisableState) {
	f.st.WithState(func(state *store.EngineState) {
		mt, ok := state.ManifestTargets[mn]
		require.Truef(f.t, ok, "manifest %s doesn't exist", mn)
		mt.State.DisableState = ds
	})
}

// waits for and returns the {num}th RmCall (1-based)
func (f *dwFixture) rmCall(num int) dockercompose.RmCall {
	require.Eventuallyf(f.t, func() bool {
		return len(f.dcClient.RmCalls()) >= num
	}, 20*time.Millisecond, time.Millisecond, "waiting for dc rm call #%d", num)
	return f.dcClient.RmCalls()[num-1]
}

// returns the names of the services stopped by the given call
func stoppedServices(call dockercompose.RmCall) []string {
	var result []string
	for _, spec := range call.Specs {
		result = append(result, spec.Service)
	}
	return result
}
