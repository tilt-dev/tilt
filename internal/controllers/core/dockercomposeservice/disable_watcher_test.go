package dockercomposeservice

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// https://app.shortcut.com/windmill/story/13147/docker-compose-down-messages-for-disabled-resources-may-be-confusing
func TestDockerComposeIgnoresGoingToRemoveMessage(t *testing.T) {
	f := newDWFixture(t)
	f.dcClient.RmOutput = `Stopping servantes_fortune_1 ...
Stopping servantes_fortune_1 ... done
servantes_fortune_1 exited with code 137
Removing servantes_fortune_1 ...
Removing servantes_fortune_1 ... done
Going to remove servantes_fortune_1
`
	f.updateQueue("m1", v1alpha1.DisableStateDisabled)
	f.clock.BlockUntil(1)
	f.clock.Advance(20 * disableDebounceDelay)
	f.startTime = f.clock.Now()

	f.log.AssertEventuallyContains(t, "Stopping servantes", time.Second)
	expectedOutput := strings.Replace(f.dcClient.RmOutput, "Going to remove servantes_fortune_1\n", "", -1)
	require.Equal(t, expectedOutput, f.log.String())
}

func TestDockerComposeDebounce(t *testing.T) {
	f := newDWFixture(t)
	f.updateQueue("m1", v1alpha1.DisableStateDisabled)
	f.updateQueue("m2", v1alpha1.DisableStateEnabled)
	require.Len(t, f.dcClient.RmCalls(), 0)

	f.updateQueue("m2", v1alpha1.DisableStateDisabled)

	f.clock.BlockUntil(2)
	f.clock.Advance(20 * disableDebounceDelay)

	call := f.rmCall(1)

	require.Equal(t, []string{"m1", "m2"}, stoppedServices(call))
}

func TestDockerComposeDontRetryOnSameStartTime(t *testing.T) {
	f := newDWFixture(t)
	f.updateQueue("m1", v1alpha1.DisableStateDisabled)
	f.updateQueue("m2", v1alpha1.DisableStateEnabled)
	require.Len(t, f.dcClient.RmCalls(), 0)

	f.updateQueue("m2", v1alpha1.DisableStateDisabled)

	f.clock.BlockUntil(2)
	f.clock.Advance(2 * disableDebounceDelay)

	call := f.rmCall(1)
	require.Equal(t, []string{"m1", "m2"}, stoppedServices(call))

	f.updateQueue("m2", v1alpha1.DisableStateDisabled)

	require.Neverf(t, func() bool {
		return len(f.dcClient.RmCalls()) > 1
	}, 20*time.Millisecond, time.Millisecond, "docker-compose should not be called again")
}

func TestDockerComposeRetryIfStartTimeChanges(t *testing.T) {
	f := newDWFixture(t)
	f.updateQueue("m1", v1alpha1.DisableStateDisabled)
	f.updateQueue("m2", v1alpha1.DisableStateEnabled)
	require.Len(t, f.dcClient.RmCalls(), 0)
	f.clock.BlockUntil(1)

	f.updateQueue("m2", v1alpha1.DisableStateDisabled)
	f.clock.BlockUntil(2)

	f.clock.Advance(2 * disableDebounceDelay)

	require.Eventually(t, func() bool {
		return len(f.dcClient.RmCalls()) == 1
	}, time.Second, 10*time.Millisecond, "docker-compose rm called")

	call := f.rmCall(1)
	require.Equal(t, []string{"m1", "m2"}, stoppedServices(call))

	// simulate restarting m2 by bumping its start time
	f.startTime = f.clock.Now()
	f.updateQueue("m2", v1alpha1.DisableStateDisabled)

	f.clock.BlockUntil(1)
	f.clock.Advance(2 * disableDebounceDelay)

	call = f.rmCall(2)
	require.Equal(t, []string{"m2"}, stoppedServices(call))
}

func TestDockerComposeDontDisableIfReenabledDuringDebounce(t *testing.T) {
	f := newDWFixture(t)
	f.updateQueue("m1", v1alpha1.DisableStateDisabled)
	f.updateQueue("m2", v1alpha1.DisableStateDisabled)

	f.clock.BlockUntil(2)

	// reenable m2 during debounce
	f.updateQueue("m2", v1alpha1.DisableStateEnabled)

	f.clock.Advance(2 * disableDebounceDelay)

	call := f.rmCall(1)

	require.Equal(t, []string{"m1"}, stoppedServices(call))
}

func TestDisableError(t *testing.T) {
	f := newDWFixture(t)

	f.dcClient.RmError = errors.New("fake dc error")
	f.updateQueue("m1", v1alpha1.DisableStateDisabled)

	f.clock.BlockUntil(1)
	f.clock.Advance(2 * disableDebounceDelay)

	require.Eventually(t, func() bool {
		return strings.Contains(f.log.String(), "fake dc error")
	}, 20*time.Millisecond, time.Millisecond)
}

// Iterations of this subscriber have spawned goroutines for every update call, so try to
// verify it's not doing that.
func TestDontSpawnRedundantGoroutines(t *testing.T) {
	f := newDWFixture(t)
	f.updateQueue("m1", v1alpha1.DisableStateDisabled)
	f.updateQueue("m2", v1alpha1.DisableStateDisabled)

	for i := 0; i < 10; i++ {
		f.updateQueue("m1", v1alpha1.DisableStateDisabled)
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

	f.clock.Advance(20 * disableDebounceDelay)

	call := f.rmCall(1)

	require.Equal(t, []string{"m1", "m2"}, stoppedServices(call))
}

type dwFixture struct {
	*tempdir.TempDirFixture
	t         *testing.T
	ctx       context.Context
	dcClient  *dockercompose.FakeDCClient
	watcher   *DisableSubscriber
	clock     clockwork.FakeClock
	log       *bufsync.ThreadSafeBuffer
	startTime time.Time
}

func newDWFixture(t *testing.T) *dwFixture {
	log := bufsync.NewThreadSafeBuffer()
	out := io.MultiWriter(log, os.Stdout)
	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(out))
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	dcClient := dockercompose.NewFakeDockerComposeClient(t, ctx)
	clock := clockwork.NewFakeClock()
	watcher := NewDisableSubscriber(ctx, dcClient, clock)

	return &dwFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
		dcClient:       dcClient,
		watcher:        watcher,
		clock:          clock,
		log:            log,
		startTime:      clock.Now(),
	}
}

func (f *dwFixture) updateQueue(mn model.ManifestName, disableState v1alpha1.DisableState) {
	m := manifestbuilder.New(f, mn).WithDockerCompose().Build()
	f.watcher.UpdateQueue(resourceState{
		Name:         mn.String(),
		Spec:         m.DockerComposeTarget().Spec,
		NeedsCleanup: disableState == v1alpha1.DisableStateDisabled,
		StartTime:    f.startTime,
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
