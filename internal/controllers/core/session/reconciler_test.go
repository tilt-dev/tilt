package session

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/sessions"
	"github.com/tilt-dev/tilt/internal/store/tiltfiles"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var sessionKey = types.NamespacedName{Name: sessions.DefaultSessionName}

func TestExitControlCI_TiltfileFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	// Tiltfile state is stored independent of resource state within engine
	f.Store.WithState(func(state *store.EngineState) {
		ms := &store.ManifestState{}
		ms.AddCompletedBuild(model.BuildRecord{
			Error: errors.New("fake Tiltfile error"),
		})
		state.TiltfileStates[model.MainTiltfileManifestName] = ms
	})

	f.MustReconcile(sessionKey)
	f.requireDoneWithError("fake Tiltfile error")
}

func TestExitControlIdempotent(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	f.MustReconcile(sessionKey)

	var s1 v1alpha1.Session
	f.MustGet(sessionKey, &s1)

	f.MustReconcile(sessionKey)

	var s2 v1alpha1.Session
	f.MustGet(sessionKey, &s2)

	assert.Equal(t, s1.ObjectMeta, s2.ObjectMeta)
}

func TestExitControlCI_FirstBuildFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m)
	m2 := manifestbuilder.New(f, "fe2").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m2)

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.Store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
			Error:      fmt.Errorf("does not compile"),
		})
	})

	f.MustReconcile(sessionKey)
	f.requireDoneWithError("does not compile")
}

func TestExitControlCI_FirstRuntimeFailure(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m)
	m2 := manifestbuilder.New(f, "fe2").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m2)
	f.Store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
		state.ManifestTargets["fe2"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
	})

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.Store.WithState(func(state *store.EngineState) {
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

	f.MustReconcile(sessionKey)
	f.requireDoneWithError("Pod pod-a in error state due to container c1: ErrImagePull")
}

func TestExitControlCI_GracePeriod(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	var session v1alpha1.Session
	f.MustGet(types.NamespacedName{Name: "Tiltfile"}, &session)
	session.Spec.CI = &v1alpha1.SessionCISpec{K8sGracePeriod: &metav1.Duration{Duration: time.Minute}}
	f.Update(&session)

	f.upsertFailingPod("fe")

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.clock.Advance(50 * time.Second)

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.clock.Advance(20 * time.Second)
	f.MustReconcile(sessionKey)
	f.requireDoneWithError("exceeded grace period: Pod pod-a in error state due to container c1: ErrImagePull")
}

func TestExitControlCI_Timeout(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	var session v1alpha1.Session
	f.MustGet(types.NamespacedName{Name: "Tiltfile"}, &session)
	session.Spec.CI = &v1alpha1.SessionCISpec{Timeout: &metav1.Duration{Duration: time.Minute}}
	f.Update(&session)

	m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m)

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.clock.Advance(50 * time.Second)

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.clock.Advance(20 * time.Second)
	f.MustReconcile(sessionKey)
	f.requireDoneWithError("Timeout after 1m0s: 2 resources waiting (fe:runtime waiting-for-pod,fe:update waiting-for-cluster)")
}

func TestExitControlCI_PodRunningContainerError(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m)
	f.Store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
	})

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.Store.WithState(func(state *store.EngineState) {
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

	f.MustReconcile(sessionKey)
	// even though one of the containers is in an error state, CI shouldn't exit - expectation is that the target for
	// the pod is in Waiting state
	f.requireNotDone()

	f.Store.WithState(func(state *store.EngineState) {
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

	f.MustReconcile(sessionKey)
	f.requireDoneWithNoError()
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

	f.Store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
		state.ManifestTargets["fe2"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
		// pod-a: ready / pod-b: doesn't exist
		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeStateWithPods(m, f.pod(k8s.PodID("pod-a"), true))
	})

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	// pod-a: ready / pod-b: ready
	f.Store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe2"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, f.pod(k8s.PodID("pod-b"), true))
	})

	f.MustReconcile(sessionKey)
	f.requireDoneWithNoError()
}

func TestExitControlCI_PodReadinessMode_Wait(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.SanchoYAML).
		WithK8sPodReadiness(model.PodReadinessWait).
		Build()
	f.upsertManifest(m)
	f.Store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})

		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeStateWithPods(m,
			f.pod(k8s.PodID("pod-a"), false))
	})

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.Store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
			f.pod(k8s.PodID("pod-a"), true),
		)
	})

	f.MustReconcile(sessionKey)
	f.requireDoneWithNoError()
}

// TestExitControlCI_PodReadinessMode_Ignore_Pods covers the case where you don't care about a Pod's readiness state
func TestExitControlCI_PodReadinessMode_Ignore_Pods(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.SecretYaml).
		WithK8sPodReadiness(model.PodReadinessIgnore).
		Build()
	f.upsertManifest(m)
	f.Store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})

		// created but no pods yet
		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeState(m)
	})

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.Store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		// pod deployed, but explicitly not ready - we should not care and exit anyway
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, f.pod(k8s.PodID("pod-a"), false))
	})

	f.MustReconcile(sessionKey)
	f.requireDoneWithNoError()
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
	f.Store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})

		state.ManifestTargets["fe"].State.RuntimeState = store.NewK8sRuntimeState(m)
	})

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.Store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		krs := store.NewK8sRuntimeState(mt.Manifest)
		// entities were created, but there's no pods in sight!
		krs.HasEverDeployedSuccessfully = true
		mt.State.RuntimeState = krs
	})

	f.MustReconcile(sessionKey)
	f.requireDoneWithNoError()
}

func TestExitControlCI_JobSuccess(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.JobYAML).
		WithK8sPodReadiness(model.PodReadinessSucceeded).
		Build()
	f.upsertManifest(m)
	f.Store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
		krs := store.NewK8sRuntimeStateWithPods(m, f.pod(k8s.PodID("pod-a"), true))
		state.ManifestTargets["fe"].State.RuntimeState = krs
	})

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.Store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		krs := store.NewK8sRuntimeStateWithPods(mt.Manifest, successPod("pod-a"))
		mt.State.RuntimeState = krs
	})

	f.MustReconcile(sessionKey)
	f.requireDoneWithNoError()
}

func TestExitControlCI_JobSuccessWithNoPods(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	m := manifestbuilder.New(f, "fe").
		WithK8sYAML(testyaml.JobYAML).
		WithK8sPodReadiness(model.PodReadinessSucceeded).
		Build()
	f.upsertManifest(m)
	f.Store.WithState(func(state *store.EngineState) {
		state.ManifestTargets["fe"].State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
	})

	f.MustReconcile(sessionKey)
	f.requireNotDone()

	f.Store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		krs := store.NewK8sRuntimeState(mt.Manifest)
		krs.HasEverDeployedSuccessfully = true
		// There are no pods but the job completed successfully.
		krs.Conditions = []metav1.Condition{
			{
				Type:   v1alpha1.ApplyConditionJobComplete,
				Status: metav1.ConditionTrue,
			},
		}
		mt.State.RuntimeState = krs
	})

	f.MustReconcile(sessionKey)
	f.requireDoneWithNoError()
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
				f.MustReconcile(sessionKey)
				f.requireNotDone()

				// N.B. a build is triggered regardless of if there is an update_cmd! it's a fake build produced
				// 	by the engine in this case, which is why this test doesn't have cases for empty update_cmd
				f.Store.WithState(func(state *store.EngineState) {
					mt := state.ManifestTargets["fe"]
					mt.State.AddCompletedBuild(model.BuildRecord{
						StartTime:  f.clock.Now(),
						FinishTime: f.clock.Now(),
					})
				})

				if tc.serveCmd {
					// the serve_cmd hasn't started yet, so no exit signal should be received still even though
					// a build occurred
					f.MustReconcile(sessionKey)
					f.requireNotDone()

					// only mimic a runtime state if there is a serve_cmd since this won't be populated
					// otherwise
					f.Store.WithState(func(state *store.EngineState) {
						mt := state.ManifestTargets["fe"]
						mt.State.RuntimeState = store.LocalRuntimeState{
							CmdName:                  "echo hi",
							Status:                   v1alpha1.RuntimeStatusOK,
							PID:                      1234,
							StartTime:                f.clock.Now(),
							LastReadyOrSucceededTime: f.clock.Now(),
							Ready:                    true,
						}
					})
				}
			}

			// for auto_init=True, it's now ready, so can exit
			// for auto_init=False, it should NOT block on it, so can exit
			f.MustReconcile(sessionKey)
			f.requireDoneWithNoError()
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
				f.MustReconcile(sessionKey)
				f.requireNotDone()

				f.Store.WithState(func(state *store.EngineState) {
					mt := state.ManifestTargets["fe"]
					mt.State.AddCompletedBuild(model.BuildRecord{
						StartTime:  f.clock.Now(),
						FinishTime: f.clock.Now(),
					})
					mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, successPod("pod-a"))
				})
			}

			// for auto_init=True, it's now ready, so can exit
			// for auto_init=False, it should NOT block on it, so can exit
			f.MustReconcile(sessionKey)
			f.requireDoneWithNoError()
		})
	}
}

func TestExitControlCI_Disabled(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	f.Store.WithState(func(state *store.EngineState) {
		m1 := manifestbuilder.New(f, "m1").WithLocalServeCmd("m1").Build()
		mt1 := store.NewManifestTarget(m1)
		mt1.State.DisableState = v1alpha1.DisableStateDisabled
		state.UpsertManifestTarget(mt1)

		m2 := manifestbuilder.New(f, "m2").WithLocalResource("m2", nil).Build()
		mt2 := store.NewManifestTarget(m2)
		mt2.State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
		mt2.State.DisableState = v1alpha1.DisableStateEnabled
		state.UpsertManifestTarget(mt2)
	})

	// the manifest is disabled, so we should be ready to exit
	f.MustReconcile(sessionKey)
	f.requireDoneWithNoError()
}

func TestStatusDisabled(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	f.Store.WithState(func(state *store.EngineState) {
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

	f.MustReconcile(sessionKey)
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

func TestRequeueLongGracePeriod(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	var session v1alpha1.Session
	f.MustGet(types.NamespacedName{Name: "Tiltfile"}, &session)
	session.Spec.CI = &v1alpha1.SessionCISpec{
		Timeout:        &metav1.Duration{Duration: time.Minute},
		K8sGracePeriod: &metav1.Duration{Duration: 10 * time.Minute},
	}
	f.Update(&session)

	f.upsertFailingPod("fe")

	result, err := f.Reconcile(sessionKey)
	require.NoError(t, err)
	assert.Equal(t, time.Minute, result.RequeueAfter)

	f.clock.Advance(50 * time.Second)

	result, err = f.Reconcile(sessionKey)
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter)
}

func TestRequeueLongTimeout(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	var session v1alpha1.Session
	f.MustGet(types.NamespacedName{Name: "Tiltfile"}, &session)
	session.Spec.CI = &v1alpha1.SessionCISpec{
		Timeout:        &metav1.Duration{Duration: 10 * time.Minute},
		K8sGracePeriod: &metav1.Duration{Duration: time.Minute},
	}
	f.Update(&session)

	f.upsertFailingPod("fe")

	result, err := f.Reconcile(sessionKey)
	require.NoError(t, err)
	assert.Equal(t, time.Minute, result.RequeueAfter)

	f.clock.Advance(50 * time.Second)

	result, err = f.Reconcile(sessionKey)
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, result.RequeueAfter)
}

type fixture struct {
	*fake.ControllerFixture
	tf    *tempdir.TempDirFixture
	r     *Reconciler
	clock *clockwork.FakeClock
}

func newFixture(t testing.TB, engineMode store.EngineMode) *fixture {
	// Fake clock is set to 2006-01-02 15:04:05
	// This helps ensure that nanosecond rounding in time doesn't break tests.
	clock := clockwork.NewFakeClockAt(time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC))

	cfb := fake.NewControllerFixtureBuilder(t)
	tdf := tempdir.NewTempDirFixture(t)
	st := cfb.Store
	mn := model.MainTiltfileManifestName
	tf := &v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{Name: mn.String()},
		Spec:       v1alpha1.TiltfileSpec{Path: tdf.JoinPath("Tiltfile")},
	}
	st.WithState(func(state *store.EngineState) {
		tiltfiles.HandleTiltfileUpsertAction(state, tiltfiles.TiltfileUpsertAction{
			Tiltfile: tf,
		})
		state.TiltfileStates[mn].AddCompletedBuild(model.BuildRecord{
			StartTime:  clock.Now(),
			FinishTime: clock.Now(),
			Reason:     model.BuildReasonFlagInit,
		})
	})

	r := NewReconciler(cfb.Client, st, clock)
	cf := cfb.Build(r)

	session := sessions.FromTiltfile(tf, nil, model.CITimeoutFlag(model.CITimeoutDefault), engineMode)
	session.Status.StartTime = apis.NewMicroTime(clock.Now())
	cf.Create(session)

	return &fixture{
		ControllerFixture: cf,
		tf:                tdf,
		r:                 r,
		clock:             clock,
	}
}

func (f *fixture) upsertManifest(m model.Manifest) {
	f.Store.WithState(func(state *store.EngineState) {
		mt := store.NewManifestTarget(m)
		mt.State.DisableState = v1alpha1.DisableStateEnabled
		state.UpsertManifestTarget(mt)
	})
}

func (f *fixture) sessionStatus() v1alpha1.SessionStatus {
	f.T().Helper()
	var session v1alpha1.Session
	f.MustGet(types.NamespacedName{Name: "Tiltfile"}, &session)
	return session.Status
}

func (f *fixture) requireNotDone() {
	f.T().Helper()
	require.False(f.T(), f.sessionStatus().Done)
}

func (f *fixture) requireDoneWithError(errString string) {
	f.T().Helper()
	status := f.sessionStatus()
	assert.True(f.T(), status.Done)
	require.Equal(f.T(), status.Error, errString)
}

func (f *fixture) requireDoneWithNoError() {
	f.T().Helper()
	status := f.sessionStatus()
	assert.True(f.T(), status.Done)
	require.Equal(f.T(), status.Error, "")
}

func (f *fixture) JoinPath(path ...string) string {
	return f.tf.JoinPath(path...)
}
func (f *fixture) MkdirAll(path string) {
	f.tf.MkdirAll(path)
}
func (f *fixture) Path() string {
	return f.tf.Path()
}

func (f *fixture) upsertFailingPod(mn model.ManifestName) {
	m := manifestbuilder.New(f, mn).WithK8sYAML(testyaml.SanchoYAML).Build()
	f.upsertManifest(m)
	f.Store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets[mn]
		mt.State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
		mt.State.LastSuccessfulDeployTime = f.clock.Now()
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, v1alpha1.Pod{
			Name:   "pod-a",
			Status: "ErrImagePull",
			Containers: []v1alpha1.Container{
				{
					Name: "c1",
					State: v1alpha1.ContainerState{
						Terminated: &v1alpha1.ContainerStateTerminated{
							StartedAt:  apis.NewTime(f.clock.Now()),
							FinishedAt: apis.NewTime(f.clock.Now()),
							Reason:     "Error",
							ExitCode:   127,
						},
					},
				},
			},
		})
	})
}

func (f *fixture) pod(podID k8s.PodID, ready bool) v1alpha1.Pod {
	return v1alpha1.Pod{
		Name:  podID.String(),
		Phase: string(v1.PodRunning),
		Containers: []v1alpha1.Container{
			{
				ID:    string(podID + "-container"),
				Ready: ready,
				State: v1alpha1.ContainerState{
					Running: &v1alpha1.ContainerStateRunning{StartedAt: metav1.NewTime(f.clock.Now())},
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

func TestExitControlCI_ReadinessTimeout(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	// Set the session CI spec with a ReadinessTimeout of 30 seconds.
	var session v1alpha1.Session
	f.MustGet(types.NamespacedName{Name: "Tiltfile"}, &session)
	session.Spec.CI = &v1alpha1.SessionCISpec{
		ReadinessTimeout: &metav1.Duration{Duration: 30 * time.Second},
	}
	f.Update(&session)

	// Upsert a manifest with K8sPodReadiness set to Wait so that update target is created.
	m := manifestbuilder.New(f, "fe").
		WithK8sPodReadiness(model.PodReadinessWait).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	f.upsertManifest(m)

	// Simulate a build and a runtime state with a pod that is not ready.
	f.Store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["fe"]
		mt.State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
		// Set runtime state with a pod that is not ready.
		mt.State.LastSuccessfulDeployTime = f.clock.Now()
		mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, f.pod(k8s.PodID("pod-test"), false))
	})

	// Initial reconcile: the readiness timeout should not have been reached yet.
	f.MustReconcile(sessionKey)
	f.requireNotDone()

	// Advance the clock by 35 seconds to exceed the 30-second readiness timeout.
	f.clock.Advance(35 * time.Second)
	f.MustReconcile(sessionKey)
	// Expect the CI job to fail with a readiness timeout error.
	// The expected error message is built as:
	// "Readiness timeout after 30s: target fe:update not ready"
	f.requireDoneWithError("Readiness timeout after 30s: target fe:runtime not ready")
}

func TestExitControlCI_LocalReadinessTimeout(t *testing.T) {
	f := newFixture(t, store.EngineModeCI)

	// Set the session CI spec with a ReadinessTimeout of 30 seconds.
	var session v1alpha1.Session
	f.MustGet(types.NamespacedName{Name: "Tiltfile"}, &session)
	session.Spec.CI = &v1alpha1.SessionCISpec{
		ReadinessTimeout: &metav1.Duration{Duration: 30 * time.Second},
	}
	f.Update(&session)

	// Upsert a local resource with a serve command so that the serve target is created.
	m := manifestbuilder.New(f, "local").
		WithLocalResource("echo hi", nil).
		WithLocalServeCmd("sleep 9999").
		Build()
	f.upsertManifest(m)

	// Simulate a build and a runtime state where the local resource is not ready.
	f.Store.WithState(func(state *store.EngineState) {
		mt := state.ManifestTargets["local"]
		mt.State.AddCompletedBuild(model.BuildRecord{
			StartTime:  f.clock.Now(),
			FinishTime: f.clock.Now(),
		})
		// Set a local runtime state that is not ready.
		mt.State.RuntimeState = store.LocalRuntimeState{
			CmdName:                  "echo hi",
			Status:                   "Not Ready",
			PID:                      123,
			StartTime:                f.clock.Now(),
			LastReadyOrSucceededTime: time.Time{}, // zero value indicates not ready
			Ready:                    false,
		}
	})

	// Initial reconcile: readiness timeout should not have been reached.
	f.MustReconcile(sessionKey)
	f.requireNotDone()

	// Advance the clock by 35 seconds to exceed the 30-second readiness timeout.
	f.clock.Advance(35 * time.Second)
	f.MustReconcile(sessionKey)

	// Expect the CI job to fail with a readiness timeout error.
	// The expected error message is:
	// "Readiness timeout after 30s: target local:serve not ready"
	f.requireDoneWithError("Readiness timeout after 30s: target local:serve not ready")
}
