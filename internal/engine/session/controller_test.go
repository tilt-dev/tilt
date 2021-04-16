package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/internal/controllers/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestExitControlCI_TiltfileFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
	defer f.TearDown()

	// Tiltfile state is stored independent of resource state within engine
	f.store.WithState(func(state *store.EngineState) {
		state.TiltfileState = &store.ManifestState{}
		state.TiltfileState.AddCompletedBuild(model.BuildRecord{
			Error: errors.New("fake Tiltfile error"),
		})
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithError("fake Tiltfile error")
}

func TestExitControlIdempotent(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
	defer f.TearDown()

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.NotNil(t, f.store.LastAction())

	f.store.ClearLastAction()
	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Nil(t, f.store.LastAction())
}

func TestExitControlCI_FirstBuildFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))

		m2 := manifestbuilder.New(f, "fe2").WithK8sYAML(testyaml.SanchoYAML).Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m2))
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
			Error:      fmt.Errorf("does not compile"),
		})
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithError("does not compile")
}

func TestExitControlCI_FirstRuntimeFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
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
		state.ManifestTargets["fe2"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, store.Pod{
			PodID:  "pod-a",
			Status: "ErrImagePull",
			Containers: []store.Container{
				store.Container{Name: "c1", Status: model.RuntimeStatusError},
			},
		})
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithError("Pod pod-a in error state due to container c1: ErrImagePull")
}

func TestExitControlCI_PodRunningContainerError(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))

		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, store.Pod{
			PodID: "pod-a",
			Phase: v1.PodRunning,
			Containers: []store.Container{
				{Name: "c1", Running: false, Ready: false, Terminated: false, Restarts: 400, Status: model.RuntimeStatusError},
				{Name: "c2", Running: true, Ready: true, Terminated: false, Status: model.RuntimeStatusOK},
			},
		})
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	// even though one of the containers is in an error state, CI shouldn't exit - expectation is that the target for
	// the pod is in Waiting state
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, store.Pod{
			PodID: "pod-a",
			Phase: v1.PodRunning,
			Containers: []store.Container{
				{Name: "c1", Running: true, Ready: true, Terminated: false, Restarts: 401, Status: model.RuntimeStatusOK},
				{Name: "c2", Running: true, Ready: true, Terminated: false, Status: model.RuntimeStatusOK},
			},
		})
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

func TestExitControlCI_Success(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").
			WithK8sYAML(testyaml.SanchoYAML).
			WithK8sPodReadiness(model.PodReadinessWait).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))

		m2 := manifestbuilder.New(f, "fe2").
			WithK8sYAML(testyaml.SanchoYAML).
			WithK8sPodReadiness(model.PodReadinessWait).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m2))

		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
		state.ManifestTargets["fe2"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
		// pod-a: ready / pod-b: doesn't exist
		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeStateWithPods(m, pod("pod-a", true))
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	// pod-a: ready / pod-b: ready
	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe2"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, pod("pod-b", true))
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

func TestExitControlCI_PodReadinessMode_Wait(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").
			WithK8sYAML(testyaml.SanchoYAML).
			WithK8sPodReadiness(model.PodReadinessWait).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))

		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})

		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeStateWithPods(m,
			pod("pod-a", false))
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
			pod("pod-a", true),
		)
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

// TestExitControlCI_PodReadinessMode_Ignore_Pods covers the case where you don't care about a Pod's readiness state
func TestExitControlCI_PodReadinessMode_Ignore_Pods(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").
			WithK8sYAML(testyaml.SecretYaml).
			WithK8sPodReadiness(model.PodReadinessIgnore).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))

		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})

		// created but no pods yet
		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeState(m)
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		// pod deployed, but explicitly not ready - we should not care and exit anyway
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, pod("pod-a", false))
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

// TestExitControlCI_PodReadinessMode_Ignore_NoPods covers the case where there are K8s resources that have no
// runtime component (i.e. no pods) - this most commonly happens with "uncategorized"
func TestExitControlCI_PodReadinessMode_Ignore_NoPods(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").
			WithK8sYAML(testyaml.SecretYaml).
			WithK8sPodReadiness(model.PodReadinessIgnore).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))

		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})

		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeState(m)
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		krs := store.NewK8sRuntimeState(mt.Manifest)
		// entities were created, but there's no pods in sight!
		krs.HasEverDeployedSuccessfully = true
		mt.State.RuntimeState = krs
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

func TestExitControlCI_JobSuccess(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)
	defer f.TearDown()

	f.store.WithState(func(state *store.EngineState) {
		m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.JobYAML).Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))

		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeStateWithPods(m, pod("pod-a", true))
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, successPod("pod-a"))
	})

	f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

func TestExitControlCI_TriggerMode_Local(t *testing.T) {
	for triggerMode := range model.TriggerModes {
		t.Run(triggerModeString(triggerMode), func(t *testing.T) {
			f := newFixture(t, store.EngineModeCI)
			defer f.TearDown()

			f.store.WithState(func(state *store.EngineState) {
				manifest := manifestbuilder.New(f, "fe").
					WithLocalResource("echo hi", nil).
					WithTriggerMode(triggerMode).Build()
				state.UpsertManifestTarget(store.NewManifestTarget(manifest))
			})

			if triggerMode.AutoInitial() {
				// because this resource SHOULD start automatically, no exit signal should be received until it's ready
				f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
				f.store.requireNoExitSignal()

				f.store.WithState(func(state *store.EngineState) {
					mt := state.ManifestTargets["fe"]
					mt.State.AddCompletedBuild(model.BuildRecord{
						StartTime:  time.Now(),
						FinishTime: time.Now(),
					})
					mt.State.RuntimeState = store.LocalRuntimeState{
						CmdName:                  "echo hi",
						Status:                   model.RuntimeStatusOK,
						PID:                      1234,
						StartTime:                time.Now(),
						LastReadyOrSucceededTime: time.Now(),
						Ready:                    true,
					}
				})
			}

			// for auto_init=True, it's now ready, so can exit
			// for auto_init=False, it should NOT block on it, so can exit
			f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
			f.store.requireExitSignalWithNoError()
		})
	}
}

func TestExitControlCI_TriggerMode_K8s(t *testing.T) {
	for triggerMode := range model.TriggerModes {
		t.Run(triggerModeString(triggerMode), func(t *testing.T) {
			f := newFixture(t, store.EngineModeCI)
			defer f.TearDown()

			f.store.WithState(func(state *store.EngineState) {
				manifest := manifestbuilder.New(f, "fe").
					WithK8sYAML(testyaml.JobYAML).
					WithTriggerMode(triggerMode).
					Build()
				state.UpsertManifestTarget(store.NewManifestTarget(manifest))
			})

			if triggerMode.AutoInitial() {
				// because this resource SHOULD start automatically, no exit signal should be received until it's ready
				f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
				f.store.requireNoExitSignal()

				f.store.WithState(func(state *store.EngineState) {
					mt := state.ManifestTargets["fe"]
					mt.State.AddCompletedBuild(model.BuildRecord{
						StartTime:  time.Now(),
						FinishTime: time.Now(),
					})
					mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, successPod("pod-a"))
				})
			}

			// for auto_init=True, it's now ready, so can exit
			// for auto_init=False, it should NOT block on it, so can exit
			f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
			f.store.requireExitSignalWithNoError()
		})
	}
}

type fixture struct {
	*tempdir.TempDirFixture
	ctx   context.Context
	store *testStore
	c     *Controller
}

func newFixture(t *testing.T, engineMode store.EngineMode) *fixture {
	f := tempdir.NewTempDirFixture(t)

	st := NewTestingStore(t)
	st.WithState(func(state *store.EngineState) {
		state.EngineMode = engineMode
		state.TiltfilePath = f.JoinPath("Tiltfile")
		state.TiltfileState.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
			Reason:     model.BuildReasonFlagInit,
		})
	})

	cli := fake.NewTiltClient()
	c := NewController(cli)
	ctx := context.Background()
	l := logger.NewLogger(logger.VerboseLvl, os.Stdout)
	ctx = logger.WithLogger(ctx, l)

	return &fixture{
		TempDirFixture: f,
		ctx:            ctx,
		store:          st,
		c:              c,
	}
}

type testStore struct {
	*store.TestingStore
	t testing.TB

	mu         sync.Mutex
	lastAction store.Action
}

func NewTestingStore(t testing.TB) *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		t:            t,
	}
}

func (s *testStore) LastAction() store.Action {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastAction
}

func (s *testStore) ClearLastAction() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastAction = nil
}

func (s *testStore) Dispatch(action store.Action) {
	s.mu.Lock()
	s.lastAction = action
	s.mu.Unlock()

	s.TestingStore.Dispatch(action)

	a, ok := action.(SessionUpdateStatusAction)
	if ok {
		state := s.LockMutableStateForTesting()
		HandleSessionUpdateStatusAction(state, a)
		s.UnlockMutableState()
	}
}

func (s *testStore) requireNoExitSignal() {
	s.t.Helper()
	state := s.RLockState()
	defer s.RUnlockState()
	require.Falsef(s.t, state.ExitSignal, "ExitSignal was not false, ExitError=%v", state.ExitError)
}

func (s *testStore) requireExitSignalWithError(errString string) {
	s.t.Helper()
	state := s.RLockState()
	defer s.RUnlockState()
	require.EqualError(s.t, state.ExitError, errString)
	assert.True(s.t, state.ExitSignal, "ExitSignal was not true")
}

func (s *testStore) requireExitSignalWithNoError() {
	s.t.Helper()
	state := s.RLockState()
	defer s.RUnlockState()
	require.True(s.t, state.ExitSignal, "ExitSignal was not true")
	require.NoError(s.t, state.ExitError)
}

func pod(podID k8s.PodID, ready bool) store.Pod {
	return store.Pod{
		PodID: podID,
		Phase: v1.PodRunning,
		Containers: []store.Container{
			store.Container{
				ID:    container.ID(podID + "-container"),
				Ready: ready,
			},
		},
	}
}

func successPod(podID k8s.PodID) store.Pod {
	return store.Pod{
		PodID:  podID,
		Phase:  v1.PodSucceeded,
		Status: "Completed",
		Containers: []store.Container{
			store.Container{
				ID: container.ID(podID + "-container"),
			},
		},
	}
}

func triggerModeString(v model.TriggerMode) string {
	switch v {
	case model.TriggerModeAuto:
		return "TriggerModeAuto"
	case model.TriggerModeAutoWithManualInit:
		return "TriggerModeAutoWithManualInit"
	case model.TriggerModeManual:
		return "TriggerModeManual"
	case model.TriggerModeManualWithAutoInit:
		return "TriggerModeManualWithAutoInit"
	default:
		panic(fmt.Errorf("unknown trigger mode value: %v", v))
	}
}
