package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/logger"
)

func newCtx() context.Context {
	return logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stderr))
}

func TestSubscriber(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := newCtx()
	s := newFakeSubscriber()
	require.NoError(t, st.AddSubscriber(ctx, s))

	st.NotifySubscribers(ctx, ChangeSummary{Legacy: true})
	call := <-s.onChange
	close(call.done)
	assert.True(t, call.summary.Legacy)
}

func TestSubscriberInterleavedCalls(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := newCtx()
	s := newFakeSubscriber()
	require.NoError(t, st.AddSubscriber(ctx, s))

	st.NotifySubscribers(ctx, ChangeSummary{})
	call := <-s.onChange
	st.NotifySubscribers(ctx, LegacyChangeSummary())
	st.NotifySubscribers(ctx, LegacyChangeSummary())
	time.Sleep(10 * time.Millisecond)
	close(call.done)

	call = <-s.onChange
	close(call.done)

	select {
	case <-s.onChange:
		t.Fatal("Expected no more onChange calls")
	case <-time.After(10 * time.Millisecond):
	}
}

func TestSubscriberInterleavedCallsSummary(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := newCtx()
	s := newFakeSubscriber()
	require.NoError(t, st.AddSubscriber(ctx, s))

	nn1 := types.NamespacedName{Name: "spec-1"}

	st.NotifySubscribers(ctx, ChangeSummary{CmdSpecs: NewChangeSet(nn1)})
	call := <-s.onChange
	assert.Equal(t, call.summary, ChangeSummary{CmdSpecs: NewChangeSet(nn1)})

	nn2 := types.NamespacedName{Name: "spec-2"}
	nn3 := types.NamespacedName{Name: "spec-3"}

	st.NotifySubscribers(ctx, ChangeSummary{CmdSpecs: NewChangeSet(nn2)})
	st.NotifySubscribers(ctx, ChangeSummary{CmdSpecs: NewChangeSet(nn3)})
	time.Sleep(10 * time.Millisecond)
	close(call.done)

	call = <-s.onChange
	assert.Equal(t, call.summary, ChangeSummary{CmdSpecs: NewChangeSet(nn2, nn3)})
	close(call.done)

	select {
	case <-s.onChange:
		t.Fatal("Expected no more onChange calls")
	case <-time.After(10 * time.Millisecond):
	}
}

func TestAddSubscriberToAlreadySetUpListCallsSetUp(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := newCtx()
	_ = st.subscribers.SetUp(ctx, st)

	s := newFakeSubscriber()
	require.NoError(t, st.AddSubscriber(ctx, s))

	assert.Equal(t, 1, s.setupCount)
}

func TestAddSubscriberBeforeSetupNoop(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := newCtx()

	s := newFakeSubscriber()
	require.NoError(t, st.AddSubscriber(ctx, s))

	// We haven't called SetUp on subscriber list as a whole, so won't call it on a new individual subscriber
	assert.Equal(t, 0, s.setupCount)
}

func TestRemoveSubscriber(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := newCtx()
	s := newFakeSubscriber()

	require.NoError(t, st.AddSubscriber(ctx, s))
	st.NotifySubscribers(ctx, ChangeSummary{})
	s.assertOnChangeCount(t, 1)

	err := st.RemoveSubscriber(ctx, s)
	assert.NoError(t, err)
	st.NotifySubscribers(ctx, LegacyChangeSummary())
	s.assertOnChangeCount(t, 0)
}

func TestRemoveSubscriberNotFound(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	s := newFakeSubscriber()
	ctx := newCtx()
	err := st.RemoveSubscriber(ctx, s)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Subscriber not found")
	}
}

func TestSubscriberSetup(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := newCtx()
	s := newFakeSubscriber()
	require.NoError(t, st.AddSubscriber(ctx, s))

	_ = st.subscribers.SetUp(ctx, st)

	assert.Equal(t, 1, s.setupCount)
}

func TestSubscriberTeardown(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := newCtx()
	s := newFakeSubscriber()
	require.NoError(t, st.AddSubscriber(ctx, s))

	go st.Dispatch(NewErrorAction(context.Canceled))
	err := st.Loop(ctx)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "context canceled")
	}

	assert.Equal(t, 1, s.teardownCount)
}

func TestSubscriberTeardownOnRemove(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := newCtx()
	s := newFakeSubscriber()
	require.NoError(t, st.AddSubscriber(ctx, s))

	errChan := make(chan error)
	go func() {
		err := st.Loop(ctx)
		errChan <- err
	}()

	// Make sure the loop has started.
	st.NotifySubscribers(ctx, ChangeSummary{})
	s.assertOnChangeCount(t, 1)

	// Remove the subscriber and make sure it doesn't get a change.
	_ = st.RemoveSubscriber(ctx, s)
	st.NotifySubscribers(ctx, ChangeSummary{})
	s.assertOnChangeCount(t, 0)

	assert.Equal(t, 1, s.teardownCount)

	st.Dispatch(NewErrorAction(context.Canceled))

	err := <-errChan
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "context canceled")
	}
	assert.Equal(t, 1, s.teardownCount)
}

type blockingSleeper struct {
	SleepDur chan time.Duration
}

func newBlockingSleeper() blockingSleeper {
	return blockingSleeper{SleepDur: make(chan time.Duration)}
}

func (s blockingSleeper) Sleep(ctx context.Context, d time.Duration) {
	if d == actionBatchWindow {
		return
	}
	s.SleepDur <- d
}

func TestSubscriberBackoff(t *testing.T) {
	bs := newBlockingSleeper()
	st, _ := NewStoreWithFakeReducer()
	st.sleeper = bs

	ctx := newCtx()
	s := newFakeSubscriber()
	require.NoError(t, st.AddSubscriber(ctx, s))

	// Fire a change that fails
	nn1 := types.NamespacedName{Name: "spec-1"}
	st.NotifySubscribers(ctx, ChangeSummary{CmdSpecs: NewChangeSet(nn1)})

	call := <-s.onChange
	assert.Equal(t, call.summary, ChangeSummary{CmdSpecs: NewChangeSet(nn1)})
	call.done <- fmt.Errorf("failed")

	// Fire a second change while the first change is sleeping
	nn2 := types.NamespacedName{Name: "spec-2"}
	st.NotifySubscribers(ctx, ChangeSummary{CmdSpecs: NewChangeSet(nn2)})
	time.Sleep(10 * time.Millisecond)

	// Clear the sleeper, and process the retry.
	assert.Equal(t, time.Second, <-bs.SleepDur)

	call = <-s.onChange
	assert.Equal(t, call.summary, ChangeSummary{CmdSpecs: NewChangeSet(nn1, nn2), LastBackoff: time.Second})
	call.done <- fmt.Errorf("failed")

	// Fire a third change while the retry is sleeping
	nn3 := types.NamespacedName{Name: "spec-3"}
	st.NotifySubscribers(ctx, ChangeSummary{CmdSpecs: NewChangeSet(nn3)})
	time.Sleep(10 * time.Millisecond)

	// Clear the sleeper, and process the second retry.
	assert.Equal(t, 2*time.Second, <-bs.SleepDur)

	call = <-s.onChange
	assert.Equal(t, call.summary, ChangeSummary{CmdSpecs: NewChangeSet(nn1, nn2, nn3), LastBackoff: 2 * time.Second})
	close(call.done)
}

type subscriberWithPointerReceiver struct {
}

func (s *subscriberWithPointerReceiver) OnChange(ctx context.Context, st RStore, summary ChangeSummary) error {
	return nil
}

type subscriberWithNonPointerReceiver struct {
}

func (s subscriberWithNonPointerReceiver) OnChange(ctx context.Context, st RStore, summary ChangeSummary) error {
	return nil
}

func TestSubscriberName(t *testing.T) {
	require.Equal(t, "store.subscriberWithPointerReceiver", subscriberName(&subscriberWithPointerReceiver{}))
	require.Equal(t, "store.subscriberWithNonPointerReceiver", subscriberName(subscriberWithNonPointerReceiver{}))
}
