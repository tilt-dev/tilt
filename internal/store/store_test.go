package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessActions(t *testing.T) {
	f := newFixture(t)
	f.Start()

	f.store.Dispatch(CompletedBuildAction{})
	f.store.Dispatch(CompletedBuildAction{})
	f.store.Dispatch(DoneAction{})

	f.WaitUntilDone()

	assert.Equal(t, 2, f.store.state.CompletedBuildCount)
}

func TestBroadcastActions(t *testing.T) {
	f := newFixture(t)

	s := newFakeSubscriber()
	_ = f.store.AddSubscriber(f.ctx, s)

	f.Start()

	f.store.Dispatch(CompletedBuildAction{})

	s.assertOnChangeCount(t, 1)

	f.store.Dispatch(DoneAction{})
	f.WaitUntilDone()
}

func TestLogOnly(t *testing.T) {
	f := newFixture(t)

	s := newFakeSubscriber()
	_ = f.store.AddSubscriber(f.ctx, s)

	f.Start()

	f.store.Dispatch(CompletedBuildAction{})
	call := <-s.onChange
	assert.False(t, call.summary.IsLogOnly())
	assert.True(t, call.summary.Legacy)
	close(call.done)

	f.store.Dispatch(LogAction{})
	call = <-s.onChange
	assert.True(t, call.summary.IsLogOnly())
	close(call.done)

	f.store.Dispatch(DoneAction{})
	f.WaitUntilDone()
}

func TestBroadcastActionsBatching(t *testing.T) {
	f := newFixture(t)

	s := newFakeSubscriber()
	_ = f.store.AddSubscriber(f.ctx, s)

	f.Start()

	f.store.mu.Lock()
	f.store.Dispatch(CompletedBuildAction{})
	f.store.Dispatch(CompletedBuildAction{})
	f.store.mu.Unlock()

	s.assertOnChangeCount(t, 1)

	f.store.Dispatch(DoneAction{})
	f.WaitUntilDone()
}

type fixture struct {
	t      *testing.T
	store  *Store
	ctx    context.Context
	cancel func()
	done   chan error
}

func newFixture(t *testing.T) fixture {
	ctx, cancel := context.WithCancel(context.Background())
	st := NewStore(TestReducer, LogActionsFlag(false))
	return fixture{
		t:      t,
		store:  st,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan error),
	}
}

func (f fixture) Start() {
	go func() {
		err := f.store.Loop(f.ctx)
		f.done <- err
	}()
}

func (f fixture) WaitUntilDone() {
	err := <-f.done
	if err != nil && err != context.Canceled {
		f.t.Fatalf("Loop failed unexpectedly: %v", err)
	}
}

func (f fixture) TearDown() {
	f.cancel()
	f.WaitUntilDone()
}

type CompletedBuildAction struct {
}

func (CompletedBuildAction) Action() {}

type DoneAction struct {
}

func (DoneAction) Action() {}

var TestReducer = Reducer(func(ctx context.Context, s *EngineState, action Action) {
	switch action.(type) {
	case CompletedBuildAction:
		s.CompletedBuildCount++
	case DoneAction:
		s.FatalError = context.Canceled
	}
})
