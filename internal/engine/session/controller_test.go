package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/tiltfiles"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestExitControlCI_TiltfileFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	// Tiltfile state is stored independent of resource state within engine
	f.store.WithState(func(state *store.EngineState) {
		ms := &store.ManifestState{}
		ms.AddCompletedBuild(model.BuildRecord{
			Error: errors.New("fake Tiltfile error"),
		})
		state.TiltfileStates[model.MainTiltfileManifestName] = ms
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithError("fake Tiltfile error")
}

func TestExitControlIdempotent(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.NotNil(t, f.store.LastAction())

	f.store.ClearLastAction()
	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	assert.Nil(t, f.store.LastAction())
}

func TestExitControlCI_FirstBuildFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m)
	m2 := manifestbuilder.New(f, "fe2").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m2)

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
			Error:      fmt.Errorf("does not compile"),
		})
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithError("does not compile")
}

func TestExitControlCI_FirstRuntimeFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m)
	m2 := manifestbuilder.New(f, "fe2").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m2)
	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
		state.ManifestTargets["fe2"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, v1alpha1.Pod{
			Name:   "pod-a",
			Status: "ErrImagePull",
			Containers: []v1alpha1.Container{
				{
					Name: "c1",
					State: v1alpha1.ContainerState{
						Terminated: &v1alpha1.ContainerStateTerminated{
							StartedAt:  metav1.Now(),
							FinishedAt: metav1.Now(),
							Reason:     "Error",
							ExitCode:   127,
						},
					},
				},
			},
		})
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithError("Pod pod-a in error state due to container c1: ErrImagePull")
}

func TestExitControlCI_PodRunningContainerError(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m)
	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, v1alpha1.Pod{
			Name:  "pod-a",
			Phase: string(v1.PodRunning),
			Containers: []v1alpha1.Container{
				{
					Name:     "c1",
					Ready:    false,
					Restarts: 400,
					State: v1alpha1.ContainerState{
						Terminated: &v1alpha1.ContainerStateTerminated{
							StartedAt:  metav1.Now(),
							FinishedAt: metav1.Now(),
							Reason:     "Error",
							ExitCode:   127,
						},
					},
				},
				{
					Name:  "c2",
					Ready: true,
					State: v1alpha1.ContainerState{
						Running: &v1alpha1.ContainerStateRunning{StartedAt: metav1.Now()},
					},
				},
			},
		})
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	// even though one of the containers is in an error state, CI shouldn't exit - expectation is that the target for
	// the pod is in Waiting state
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		pod := mt.State.K8sRuntimeState().GetPods()[0]
		c1 := pod.Containers[0]
		c1.Ready = true
		c1.Restarts++
		c1.State = v1alpha1.ContainerState{
			Running: &v1alpha1.ContainerStateRunning{StartedAt: metav1.Now()},
		}
		pod.Containers[0] = c1
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

func TestExitControlCI_Success(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.SanchoYAML).
		WithK8sPodReadiness(model.PodReadinessWait).
		Build()
	f.upsertManifest(m)

	m2 := manifestbuilder.New(f, "fe2").
		WithK8sYAML(testyaml.SanchoYAML).
		WithK8sPodReadiness(model.PodReadinessWait).
		Build()
	f.upsertManifest(m2)

	f.store.WithState(func(state *store.EngineState) {
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

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	// pod-a: ready / pod-b: ready
	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe2"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, pod("pod-b", true))
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

func TestExitControlCI_PodReadinessMode_Wait(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.SanchoYAML).
		WithK8sPodReadiness(model.PodReadinessWait).
		Build()
	f.upsertManifest(m)
	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})

		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeStateWithPods(m,
			pod("pod-a", false))
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
			pod("pod-a", true),
		)
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

// TestExitControlCI_PodReadinessMode_Ignore_Pods covers the case where you don't care about a Pod's readiness state
func TestExitControlCI_PodReadinessMode_Ignore_Pods(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.SecretYaml).
		WithK8sPodReadiness(model.PodReadinessIgnore).
		Build()
	f.upsertManifest(m)
	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})

		// created but no pods yet
		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeState(m)
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		// pod deployed, but explicitly not ready - we should not care and exit anyway
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, pod("pod-a", false))
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

// TestExitControlCI_PodReadinessMode_Ignore_NoPods covers the case where there are K8s resources that have no
// runtime component (i.e. no pods) - this most commonly happens with "uncategorized"
func TestExitControlCI_PodReadinessMode_Ignore_NoPods(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.SecretYaml).
		WithK8sPodReadiness(model.PodReadinessIgnore).
		Build()
	f.upsertManifest(m)
	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})

		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeState(m)
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		krs := store.NewK8sRuntimeState(mt.Manifest)
		// entities were created, but there's no pods in sight!
		krs.HasEverDeployedSuccessfully = true
		mt.State.RuntimeState = krs
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

func TestExitControlCI_JobSuccess(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.JobYAML).
		WithK8sPodReadiness(model.PodReadinessSucceeded).
		Build()
	f.upsertManifest(m)
	f.store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
		krs := store.NewK8sRuntimeStateWithPods(m, pod("pod-a", true))
		state.ManifestTargets["fe"].State.RuntimeState = krs
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireNoExitSignal()

	f.store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		krs := store.NewK8sRuntimeStateWithPods(mt.Manifest, successPod("pod-a"))
		mt.State.RuntimeState = krs
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

func TestExitControlCI_TriggerMode_Local(t *testing.T) {
	type tc struct {
		triggerMode model.TriggerMode
		serveCmd    bool
	}
	var tcs []tc
	for triggerMode := range model.TriggerModes {
		for _, hasServeCmd := range []bool{false, true} {
			tcs = append(tcs, tc{
				triggerMode: triggerMode,
				serveCmd:    hasServeCmd,
			})
		}
	}

	for _, tc := range tcs {
		name := triggerModeString(tc.triggerMode)
		if !tc.serveCmd {
			name += "_EmptyServeCmd"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, store.EngineModeCI)

			mb := manifestbuilder.New(f, "fe").
				WithLocalResource("echo hi", nil).
				WithTriggerMode(tc.triggerMode)

			if tc.serveCmd {
				mb = mb.WithLocalServeCmd("while true; echo hi; done")
			}

			f.upsertManifest(mb.Build())

			if tc.triggerMode.AutoInitial() {
				// because this resource SHOULD start automatically, no exit signal should be received before
				// a build has completed
				_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
				f.store.requireNoExitSignal()

				// N.B. a build is triggered regardless of if there is an update_cmd! it's a fake build produced
				// 	by the engine in this case, which is why this test doesn't have cases for empty update_cmd
				f.store.WithState(func(state *store.EngineState) {
					mt := state.ManifestTargets["fe"]
					mt.State.AddCompletedBuild(model.BuildRecord{
						StartTime:  time.Now(),
						FinishTime: time.Now(),
					})
				})

				if tc.serveCmd {
					// the serve_cmd hasn't started yet, so no exit signal should be received still even though
					// a build occurred
					_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
					f.store.requireNoExitSignal()

					// only mimic a runtime state if there is a serve_cmd since this won't be populated
					// otherwise
					f.store.WithState(func(state *store.EngineState) {
						mt := state.ManifestTargets["fe"]
						mt.State.RuntimeState = store.LocalRuntimeState{
							CmdName:                  "echo hi",
							Status:                   v1alpha1.RuntimeStatusOK,
							PID:                      1234,
							StartTime:                time.Now(),
							LastReadyOrSucceededTime: time.Now(),
							Ready:                    true,
						}
					})
				}
			}

			// for auto_init=True, it's now ready, so can exit
			// for auto_init=False, it should NOT block on it, so can exit
			_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
			f.store.requireExitSignalWithNoError()
		})
	}
}

func TestExitControlCI_TriggerMode_K8s(t *testing.T) {
	for triggerMode := range model.TriggerModes {
		t.Run(triggerModeString(triggerMode), func(t *testing.T) {
			f := newFixture(t, store.EngineModeCI)

			manifest := manifestbuilder.New(f, "fe").
				WithK8sYAML(testyaml.JobYAML).
				WithTriggerMode(triggerMode).
				Build()
			f.upsertManifest(manifest)

			if triggerMode.AutoInitial() {
				// because this resource SHOULD start automatically, no exit signal should be received until it's ready
				_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
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
			_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
			f.store.requireExitSignalWithNoError()
		})
	}
}

func TestExitControlCI_Disabled(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	f.store.WithState(func(state *store.EngineState) {
		m1 := manifestbuilder.New(f, "m1").WithLocalServeCmd("m1").Build()
		mt1 := store.NewManifestTarget(m1)
		mt1.State.DisableState = v1alpha1.DisableStateDisabled
		state.UpsertManifestTarget(mt1)

		m2 := manifestbuilder.New(f, "m2").WithLocalResource("m2", nil).Build()
		mt2 := store.NewManifestTarget(m2)
		mt2.State.AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
		})
		mt2.State.DisableState = v1alpha1.DisableStateEnabled
		state.UpsertManifestTarget(mt2)
	})

	// the manifest is disabled, so we should be ready to exit
	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.store.requireExitSignalWithNoError()
}

func TestStatusDisabled(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	f.store.WithState(func(state *store.EngineState) {
		m1 := manifestbuilder.New(f, "local_update").WithLocalResource("a", nil).Build()
		m2 := manifestbuilder.New(f, "local_serve").WithLocalServeCmd("a").Build()
		m3 := manifestbuilder.New(f, "k8s").WithK8sYAML(testyaml.JobYAML).Build()
		m4 := manifestbuilder.New(f, "dc").WithDockerCompose().Build()
		for _, m := range []model.Manifest{m1, m2, m3, m4} {
			mt := store.NewManifestTarget(m)
			mt.State.DisableState = v1alpha1.DisableStateDisabled
			state.UpsertManifestTarget(mt)
		}
	})

	_ = f.c.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	status := f.sessionStatus()
	targetbyName := make(map[string]v1alpha1.Target)
	for _, target := range status.Targets {
		targetbyName[target.Name] = target
	}

	expectedTargets := []string{
		"dc:runtime",
		"dc:update",
		"k8s:runtime",
		"k8s:update",
		"local_update:update",
		"local_serve:serve",
	}
	// + 1 for Tiltfile
	require.Len(t, targetbyName, len(expectedTargets)+1)
	for _, name := range expectedTargets {
		target, ok := targetbyName[name]
		require.Truef(t, ok, "no target named %q", name)
		require.NotNil(t, target.State.Disabled)
	}
}

type fixture struct {
	*tempdir.TempDirFixture
	ctx   context.Context
	store *testStore
	c     *Controller
	cli   ctrlclient.Client
}

func newFixture(t *testing.T, engineMode store.EngineMode) *fixture {
	f := tempdir.NewTempDirFixture(t)

	st := NewTestingStore(t)
	st.WithState(func(state *store.EngineState) {
		mn := model.MainTiltfileManifestName
		tiltfiles.HandleTiltfileUpsertAction(state, tiltfiles.TiltfileUpsertAction{
			Tiltfile: &v1alpha1.Tiltfile{
				ObjectMeta: metav1.ObjectMeta{Name: mn.String()},
				Spec:       v1alpha1.TiltfileSpec{Path: f.JoinPath("Tiltfile")},
			},
		})
		state.TiltfileStates[mn].AddCompletedBuild(model.BuildRecord{
			StartTime:  time.Now(),
			FinishTime: time.Now(),
			Reason:     model.BuildReasonFlagInit,
		})
	})

	cli := fake.NewFakeTiltClient()
	c := NewController(cli, engineMode)
	ctx := context.Background()
	l := logger.NewLogger(logger.VerboseLvl, os.Stdout)
	ctx = logger.WithLogger(ctx, l)

	return &fixture{
		TempDirFixture: f,
		ctx:            ctx,
		store:          st,
		c:              c,
		cli:            cli,
	}
}

func (f *fixture) upsertManifest(m model.Manifest) {
	f.store.WithState(func(state *store.EngineState) {
		mt := store.NewManifestTarget(m)
		mt.State.DisableState = v1alpha1.DisableStateEnabled
		state.UpsertManifestTarget(mt)
	})
}

func (f *fixture) sessionStatus() v1alpha1.SessionStatus {
	var session v1alpha1.Session
	err := f.cli.Get(f.ctx, types.NamespacedName{Name: "Tiltfile"}, &session)
	require.NoError(f.T(), err)
	return session.Status
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

func pod(podID k8s.PodID, ready bool) v1alpha1.Pod {
	return v1alpha1.Pod{
		Name:  podID.String(),
		Phase: string(v1.PodRunning),
		Containers: []v1alpha1.Container{
			{
				ID:    string(podID + "-container"),
				Ready: ready,
				State: v1alpha1.ContainerState{
					Running: &v1alpha1.ContainerStateRunning{StartedAt: metav1.Now()},
				},
			},
		},
	}
}

func successPod(podID k8s.PodID) v1alpha1.Pod {
	return v1alpha1.Pod{
		Name:   podID.String(),
		Phase:  string(v1.PodSucceeded),
		Status: "Completed",
		Containers: []v1alpha1.Container{
			{
				ID: string(podID + "-container"),
				State: v1alpha1.ContainerState{
					Terminated: &v1alpha1.ContainerStateTerminated{
						StartedAt:  metav1.Now(),
						FinishedAt: metav1.Now(),
						ExitCode:   0,
					},
				},
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
