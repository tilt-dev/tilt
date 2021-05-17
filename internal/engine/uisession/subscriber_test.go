package uisession

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/store"
)

func TestCreate(t *testing.T) {
	f := newFixture(t)
	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 1, len(f.store.Actions()))
	assert.Equal(t, "Tiltfile", f.store.Actions()[0].(UISessionCreateAction).UISession.Name)

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 1, len(f.store.Actions()))
}

func TestUpdate(t *testing.T) {
	f := newFixture(t)
	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 1, len(f.store.Actions()))
	assert.Equal(t, "Tiltfile", f.store.Actions()[0].(UISessionCreateAction).UISession.Name)

	f.store.WithState(func(es *store.EngineState) {
		es.CloudStatus.Username = "sparkle"
	})

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 2, len(f.store.Actions()))
	assert.Equal(t, "sparkle", f.store.Actions()[1].(UISessionUpdateStatusAction).Status.TiltCloudUsername)
}

type testStore struct {
	*store.TestingStore
}

func (s *testStore) Dispatch(a store.Action) {
	s.TestingStore.Dispatch(a)

	state := s.LockMutableStateForTesting()
	defer s.UnlockMutableState()
	switch a := a.(type) {
	case UISessionUpdateStatusAction:
		HandleUISessionUpdateStatusAction(state, a)
	case UISessionCreateAction:
		HandleUISessionCreateAction(state, a)
	}
}

type fixture struct {
	ctx   context.Context
	store *testStore
	sub   *Subscriber
}

func newFixture(t *testing.T) *fixture {
	return &fixture{
		ctx: context.Background(),
		sub: NewSubscriber(),
		store: &testStore{
			TestingStore: store.NewTestingStore(),
		},
	}
}
