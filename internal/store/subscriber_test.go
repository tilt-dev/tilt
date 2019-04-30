package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSubscriber(t *testing.T) {
	st, _ := NewStoreForTesting()
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	st.NotifySubscribers(ctx)
	call := <-s.onChange
	close(call.done)
}

func TestSubscriberInterleavedCalls(t *testing.T) {
	st, _ := NewStoreForTesting()
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
	st, _ := NewStoreForTesting()
	ctx := context.Background()
	st.subscribers.SetUp(ctx)

	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	assert.Equal(t, 1, s.setupCount)
}

func TestAddSubscriberBeforeSetupNoop(t *testing.T) {
	st, _ := NewStoreForTesting()
	ctx := context.Background()

	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	// We haven't called SetUp on subscriber list as a whole, so won't call it on a new individual subscriber
	assert.Equal(t, 0, s.setupCount)
}

func TestRemoveSubscriber(t *testing.T) {
	st, _ := NewStoreForTesting()
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
	st, _ := NewStoreForTesting()
	s := newFakeSubscriber()
	ctx := context.Background()
	err := st.RemoveSubscriber(ctx, s)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "Subscriber not found")
	}
}

func TestSubscriberSetup(t *testing.T) {
	st, _ := NewStoreForTesting()
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	st.subscribers.SetUp(ctx)

	assert.Equal(t, 1, s.setupCount)
}

func TestSubscriberTeardown(t *testing.T) {
	st, _ := NewStoreForTesting()
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(ctx, s)

	go st.Dispatch(NewErrorAction(fmt.Errorf("fake error")))
	err := st.Loop(ctx)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "fake error")
	}

	assert.Equal(t, 1, s.teardownCount)
}

func TestSubscriberTeardownOnRemove(t *testing.T) {
	st, _ := NewStoreForTesting()
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

	st.Dispatch(NewErrorAction(fmt.Errorf("fake error")))

	err := <-errChan
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "fake error")
	}
	assert.Equal(t, 1, s.teardownCount)
}

type fakeSubscriber struct {
	onChange      chan onChangeCall
	setupCount    int
	teardownCount int
}

func newFakeSubscriber() *fakeSubscriber {
	return &fakeSubscriber{
		onChange: make(chan onChangeCall),
	}
}

type onChangeCall struct {
	done chan bool
}

func (f *fakeSubscriber) assertOnChangeCount(t *testing.T, count int) {
	t.Helper()

	for i := 0; i < count; i++ {
		f.assertOnChange(t)
	}

	select {
	case <-time.After(50 * time.Millisecond):
		return

	case call := <-f.onChange:
		close(call.done)
		t.Fatalf("Expected only %d OnChange calls. Got: %d", count, count+1)
	}
}

func (f *fakeSubscriber) assertOnChange(t *testing.T) {
	t.Helper()

	select {
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for subscriber.OnChange")
	case call := <-f.onChange:
		close(call.done)
	}
}

func (f *fakeSubscriber) OnChange(ctx context.Context, st RStore) {
	call := onChangeCall{done: make(chan bool)}
	f.onChange <- call
	<-call.done
}

func (f *fakeSubscriber) SetUp(ctx context.Context) {
	f.setupCount++
}

func (f *fakeSubscriber) TearDown(ctx context.Context) {
	f.teardownCount++
}
