package exit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestExitControlAllSuccess(t *testing.T) {
	f := newFixture(t, store.EngineModeApply)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))

		m2 := manifestbuilder.New(f, "fe2").WithK8sYAML(testyaml.SanchoYAML).Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m2))

		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
	})

	f.c.OnChange(f.ctx, f.store)
	assert.False(t, f.store.exitSignal)

	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe2"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
	})

	// Verify that completing the second build causes an exit
	f.c.OnChange(f.ctx, f.store)
	assert.True(t, f.store.exitSignal)
}

func TestExitControlFirstFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeApply)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))

		m2 := manifestbuilder.New(f, "fe2").WithK8sYAML(testyaml.SanchoYAML).Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m2))
	})

	f.c.OnChange(f.ctx, f.store)
	assert.False(t, f.store.exitSignal)

	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
			Error:      fmt.Errorf("does not compile"),
		})
	})

	// Verify that if one build fails with an error, it fails immediately.
	f.c.OnChange(f.ctx, f.store)
	assert.True(t, f.store.exitSignal)
}

type fixture struct {
	*tempdir.TempDirFixture
	ctx   context.Context
	store *testStore
	c     *Controller
}

func newFixture(t *testing.T, engineMode store.EngineMode) *fixture {
	f := tempdir.NewTempDirFixture(t)

	st := NewTestingStore()
	st.WithState(func(state *store.EngineState) {
		state.EngineMode = engineMode
	})

	c := NewController()
	ctx := context.Background()

	return &fixture{
		TempDirFixture: f,
		ctx:            ctx,
		store:          st,
		c:              c,
	}
}

type testStore struct {
	*store.TestingStore
	exitSignal bool
	exitError  error
}

func NewTestingStore() *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
	}
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)

	exitAction, ok := action.(Action)
	if ok {
		s.exitSignal = exitAction.ExitSignal
		s.exitError = exitAction.ExitError
	}
}
