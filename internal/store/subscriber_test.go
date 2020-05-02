package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSubscriber(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	st.NotifySubscribers(ctx)
	call := <-s.onChange
	close(call.done)
}

func TestSubscriberInterleavedCalls(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	st.NotifySubscribers(ctx)
	call := <-s.onChange
	st.NotifySubscribers(ctx)
	st.NotifySubscribers(ctx)
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

func TestAddSubscriberToAlreadySetUpListCallsSetUp(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := context.Background()
	st.subscribers.SetUp(ctx)

	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	assert.Equal(t, 1, s.setupCount)
}

func TestAddSubscriberBeforeSetupNoop(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := context.Background()

	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	// We haven't called SetUp on subscriber list as a whole, so won't call it on a new individual subscriber
	assert.Equal(t, 0, s.setupCount)
}

func TestRemoveSubscriber(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := context.Background()
	s := newFakeSubscriber()

	st.AddSubscriber(ctx, s)
	st.NotifySubscribers(ctx)
	s.assertOnChangeCount(t, 1)

	err := st.RemoveSubscriber(ctx, s)
	assert.NoError(t, err)
	st.NotifySubscribers(ctx)
	s.assertOnChangeCount(t, 0)
}

func TestRemoveSubscriberNotFound(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	s := newFakeSubscriber()
	ctx := context.Background()
	err := st.RemoveSubscriber(ctx, s)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Subscriber not found")
	}
}

func TestSubscriberSetup(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	st.subscribers.SetUp(ctx)

	assert.Equal(t, 1, s.setupCount)
}

func TestSubscriberTeardown(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	go st.Dispatch(NewErrorAction(context.Canceled))
	err := st.Loop(ctx)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "context canceled")
	}

	assert.Equal(t, 1, s.teardownCount)
}

func TestSubscriberTeardownOnRemove(t *testing.T) {
	st, _ := NewStoreWithFakeReducer()
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	errChan := make(chan error)
	go func() {
		err := st.Loop(ctx)
		errChan <- err
	}()

	// Make sure the loop has started.
	st.NotifySubscribers(ctx)
	s.assertOnChangeCount(t, 1)

	// Remove the subscriber and make sure it doesn't get a change.
	_ = st.RemoveSubscriber(ctx, s)
	st.NotifySubscribers(ctx)
	s.assertOnChangeCount(t, 0)

	assert.Equal(t, 1, s.teardownCount)

	st.Dispatch(NewErrorAction(context.Canceled))

	err := <-errChan
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "context canceled")
	}
	assert.Equal(t, 1, s.teardownCount)
}
