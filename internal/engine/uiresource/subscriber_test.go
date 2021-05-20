package uiresource

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestCreate(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 1, len(f.store.Actions()))
	assert.Equal(t, "(Tiltfile)",
		f.store.Actions()[0].(UIResourceCreateAction).UIResource.Name)

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 1, len(f.store.Actions()))
}

func TestUpdateTiltfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 1, len(f.store.Actions()))
	assert.Equal(t, "(Tiltfile)",
		f.store.Actions()[0].(UIResourceCreateAction).UIResource.Name)

	f.store.WithState(func(es *store.EngineState) {
		es.TiltfileState.CurrentBuild.StartTime = time.Now()
	})

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 2, len(f.store.Actions()))
	assert.Equal(t, v1alpha1.UpdateStatusInProgress,
		f.store.Actions()[1].(UIResourceUpdateStatusAction).Status.UpdateStatus)
}

func TestDeleteManifest(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").
			WithK8sYAML(testyaml.SanchoYAML).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))
	})

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 2, len(f.store.Actions()))

	names := []string{
		f.store.Actions()[0].(UIResourceCreateAction).UIResource.Name,
		f.store.Actions()[1].(UIResourceCreateAction).UIResource.Name,
	}
	sort.Strings(names)
	assert.Equal(t, []string{"(Tiltfile)", "fe"}, names)

	f.store.WithState(func(state *store.EngineState) {
		state.RemoveManifestTarget("fe")
	})

	f.sub.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Equal(t, 3, len(f.store.Actions()))
	assert.Equal(t, "fe",
		f.store.Actions()[2].(UIResourceDeleteAction).Name.Name)
}

type testStore struct {
	*store.TestingStore
}

func (s *testStore) Dispatch(a store.Action) {
	s.TestingStore.Dispatch(a)

	state := s.LockMutableStateForTesting()
	defer s.UnlockMutableState()
	switch a := a.(type) {
	case UIResourceUpdateStatusAction:
		HandleUIResourceUpdateStatusAction(state, a)
	case UIResourceCreateAction:
		HandleUIResourceCreateAction(state, a)
	case UIResourceDeleteAction:
		HandleUIResourceDeleteAction(state, a)
	}
}

type fixture struct {
	*tempdir.TempDirFixture
	ctx   context.Context
	store *testStore
	sub   *Subscriber
}

func newFixture(t *testing.T) *fixture {
	tc := fake.NewTiltClient()
	return &fixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		ctx:            context.Background(),
		sub:            NewSubscriber(tc),
		store: &testStore{
			TestingStore: store.NewTestingStore(),
		},
	}
}
