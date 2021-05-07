package portforward

import (
	"context"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestPortForward(t *testing.T) {
	f := newPFSFixture(t)
	f.Start()
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{
		Name: "fe",
	}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8080,
				ContainerPort: 8081,
			},
		},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("no port forwards running yet", func(pfs map[string]*PortForward) bool {
		return len(pfs) == 0
	})

	state = f.st.LockMutableStateForTesting()
	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-A", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("one port forward for pod A", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		for _, pf := range pfs {
			assert.Equal(t, "pod-A", pf.Spec.PodName)
			f.assertOneForward(8080, 8081, pf)
		}
		return true
	})

	// assert.Equal(t, 1, len(f.s.activeForwards))
	// assert.Equal(t, "pod-A", f.kCli.LastForwardPortPodID().String())
	// firstPodForwardCtx := f.kCli.LastForwardContext()
	//
	// state = f.st.LockMutableStateForTesting()
	// mt = state.ManifestTargets["fe"]
	// mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
	// 	v1alpha1.Pod{Name: "pod-B", Phase: string(v1.PodRunning)})
	// f.st.UnlockMutableState()
	//
	// f.onChange()
	// assert.Equal(t, 1, len(f.s.activeForwards))
	// assert.Equal(t, "pod-id2", f.kCli.LastForwardPortPodID().String())
	//
	// state = f.st.LockMutableStateForTesting()
	// mt = state.ManifestTargets["fe"]
	// mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
	// 	v1alpha1.Pod{Name: "pod-id2", Phase: string(v1.PodPending)})
	// f.st.UnlockMutableState()
	//
	// f.onChange()
	// assert.Equal(t, 0, len(f.s.activeForwards))
	//
	// assert.Equal(t, context.Canceled, firstPodForwardCtx.Err(),
	// 	"Expected first port-forward to be canceled")
}

// func TestMultiplePortForwardsForOnePod(t *testing.T) {
// 	f := newPFSFixture(t)
// 	defer f.TearDown()
//
// 	state := f.st.LockMutableStateForTesting()
// 	m := model.Manifest{
// 		Name: "fe",
// 	}
// 	m = m.WithDeployTarget(model.K8sTarget{
// 		PortForwards: []model.PortForward{
// 			{
// 				LocalPort:     8000,
// 				ContainerPort: 8080,
// 			},
// 			{
// 				LocalPort:     8001,
// 				ContainerPort: 8081,
// 			},
// 		},
// 	})
// 	state.UpsertManifestTarget(store.NewManifestTarget(m))
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 0, len(f.s.activeForwards))
//
// 	state = f.st.LockMutableStateForTesting()
// 	mt := state.ManifestTargets["fe"]
// 	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
// 		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	require.Equal(t, 1, len(f.s.activeForwards))
// 	require.Equal(t, 2, f.kCli.CreatePortForwardCallCount())
//
// 	// PortForwards are executed async so we can't guarantee the order;
// 	// just make sure each expected call appears exactly once
// 	expectedRemotePorts := []int{8080, 8081}
// 	var actualRemotePorts []int
// 	var contexts []context.Context
// 	for _, call := range f.kCli.PortForwardCalls() {
// 		actualRemotePorts = append(actualRemotePorts, call.RemotePort)
// 		contexts = append(contexts, call.Context)
// 		assert.Equal(t, "pod-id", call.PodID.String())
// 	}
// 	assert.ElementsMatch(t, expectedRemotePorts, actualRemotePorts, "remote ports for which PortForward was called")
//
// 	state = f.st.LockMutableStateForTesting()
// 	mt = state.ManifestTargets["fe"]
// 	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
// 		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodPending)})
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 0, len(f.s.activeForwards))
//
// 	for _, ctx := range contexts {
// 		assert.Equal(t, context.Canceled, ctx.Err(),
// 			"found uncancelled port forward context")
// 	}
// }
//
// func TestPortForwardAutoDiscovery(t *testing.T) {
// 	f := newPFSFixture(t)
// 	defer f.TearDown()
//
// 	state := f.st.LockMutableStateForTesting()
// 	m := model.Manifest{
// 		Name: "fe",
// 	}
// 	m = m.WithDeployTarget(model.K8sTarget{
// 		PortForwards: []model.PortForward{
// 			{
// 				LocalPort: 8080,
// 			},
// 		},
// 	})
// 	state.UpsertManifestTarget(store.NewManifestTarget(m))
//
// 	mt := state.ManifestTargets["fe"]
// 	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
// 		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 0, len(f.s.activeForwards))
// 	state = f.st.LockMutableStateForTesting()
// 	state.ManifestTargets["fe"].State.K8sRuntimeState().Pods["pod-id"].Containers = []v1alpha1.Container{
// 		v1alpha1.Container{Ports: []int32{8000}},
// 	}
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 8000, f.kCli.LastForwardPortRemotePort())
// }
//
// func TestPortForwardAutoDiscovery2(t *testing.T) {
// 	f := newPFSFixture(t)
// 	defer f.TearDown()
//
// 	state := f.st.LockMutableStateForTesting()
// 	m := model.Manifest{
// 		Name: "fe",
// 	}
// 	m = m.WithDeployTarget(model.K8sTarget{
// 		PortForwards: []model.PortForward{
// 			{
// 				LocalPort: 8080,
// 			},
// 		},
// 	})
// 	state.UpsertManifestTarget(store.NewManifestTarget(m))
//
// 	mt := state.ManifestTargets["fe"]
// 	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, v1alpha1.Pod{
// 		Name:  "pod-id",
// 		Phase: string(v1.PodRunning),
// 		Containers: []v1alpha1.Container{
// 			{Ports: []int32{8000, 8080}},
// 		},
// 	})
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 8080, f.kCli.LastForwardPortRemotePort())
// }
//
// func TestPortForwardChangePort(t *testing.T) {
// 	f := newPFSFixture(t)
// 	defer f.TearDown()
//
// 	state := f.st.LockMutableStateForTesting()
// 	m := model.Manifest{Name: "fe"}.WithDeployTarget(model.K8sTarget{
// 		PortForwards: []model.PortForward{
// 			{
// 				LocalPort:     8080,
// 				ContainerPort: 8081,
// 			},
// 		},
// 	})
// 	state.UpsertManifestTarget(store.NewManifestTarget(m))
// 	mt := state.ManifestTargets["fe"]
// 	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
// 		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())
//
// 	state = f.st.LockMutableStateForTesting()
// 	kTarget := state.ManifestTargets["fe"].Manifest.K8sTarget()
// 	kTarget.PortForwards[0].ContainerPort = 8082
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 8082, f.kCli.LastForwardPortRemotePort())
// }
//
// func TestPortForwardChangeHost(t *testing.T) {
// 	f := newPFSFixture(t)
// 	defer f.TearDown()
//
// 	state := f.st.LockMutableStateForTesting()
// 	m := model.Manifest{Name: "fe"}.WithDeployTarget(model.K8sTarget{
// 		PortForwards: []model.PortForward{
// 			{
// 				LocalPort:     8080,
// 				ContainerPort: 8081,
// 				Host:          "someHostA",
// 			},
// 		},
// 	})
// 	state.UpsertManifestTarget(store.NewManifestTarget(m))
// 	mt := state.ManifestTargets["fe"]
// 	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
// 		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())
// 	assert.Equal(t, "someHostA", f.kCli.LastForwardPortHost())
//
// 	state = f.st.LockMutableStateForTesting()
// 	kTarget := state.ManifestTargets["fe"].Manifest.K8sTarget()
// 	kTarget.PortForwards[0].Host = "otherHostB"
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())
// 	assert.Equal(t, "otherHostB", f.kCli.LastForwardPortHost())
// }
//
// func TestPortForwardChangeManifestName(t *testing.T) {
// 	f := newPFSFixture(t)
// 	defer f.TearDown()
//
// 	state := f.st.LockMutableStateForTesting()
// 	m := model.Manifest{Name: "manifestA"}.WithDeployTarget(model.K8sTarget{
// 		PortForwards: []model.PortForward{
// 			{
// 				LocalPort:     8080,
// 				ContainerPort: 8081,
// 			},
// 		},
// 	})
// 	state.UpsertManifestTarget(store.NewManifestTarget(m))
// 	mt := state.ManifestTargets["manifestA"]
// 	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
// 		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())
//
// 	state = f.st.LockMutableStateForTesting()
// 	delete(state.ManifestTargets, "manifestA")
// 	m = model.Manifest{Name: "manifestB"}.WithDeployTarget(model.K8sTarget{
// 		PortForwards: []model.PortForward{
// 			{
// 				LocalPort:     8080,
// 				ContainerPort: 8081,
// 			},
// 		},
// 	})
// 	state.UpsertManifestTarget(store.NewManifestTarget(m))
// 	mt = state.ManifestTargets["manifestB"]
// 	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
// 		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())
// }
//
// func TestPortForwardRestart(t *testing.T) {
// 	if runtime.GOOS == "windows" {
// 		t.Skip("TODO(nick): investigate")
// 	}
// 	f := newPFSFixture(t)
// 	defer f.TearDown()
//
// 	state := f.st.LockMutableStateForTesting()
// 	m := model.Manifest{Name: "fe"}.WithDeployTarget(model.K8sTarget{
// 		PortForwards: []model.PortForward{
// 			{
// 				LocalPort:     8080,
// 				ContainerPort: 8081,
// 			},
// 		},
// 	})
// 	state.UpsertManifestTarget(store.NewManifestTarget(m))
// 	mt := state.ManifestTargets["fe"]
// 	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
// 		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
// 	f.st.UnlockMutableState()
//
// 	f.onChange()
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 1, f.kCli.CreatePortForwardCallCount())
//
// 	err := fmt.Errorf("unique-error")
// 	f.kCli.LastForwarder().Done <- err
//
// 	assert.Contains(t, "unique-error", f.out.String())
// 	time.Sleep(100 * time.Millisecond)
//
// 	assert.Equal(t, 1, len(f.s.activeForwards))
// 	assert.Equal(t, 2, f.kCli.CreatePortForwardCallCount())
// }
//
// type portForwardTestCase struct {
// 	spec           []model.PortForward
// 	containerPorts []int32
// 	expected       []model.PortForward
// }
//
// func TestPopulatePortForward(t *testing.T) {
// 	cases := []portForwardTestCase{
// 		{
// 			spec:           []model.PortForward{{LocalPort: 8080}},
// 			containerPorts: []int32{8080},
// 			expected:       []model.PortForward{{ContainerPort: 8080, LocalPort: 8080}},
// 		},
// 		{
// 			spec:           []model.PortForward{{LocalPort: 8080}},
// 			containerPorts: []int32{8000, 8080, 8001},
// 			expected:       []model.PortForward{{ContainerPort: 8080, LocalPort: 8080}},
// 		},
// 		{
// 			spec:           []model.PortForward{{LocalPort: 8080}, {LocalPort: 8000}},
// 			containerPorts: []int32{8000, 8080, 8001},
// 			expected: []model.PortForward{
// 				{ContainerPort: 8080, LocalPort: 8080},
// 				{ContainerPort: 8000, LocalPort: 8000},
// 			},
// 		},
// 		{
// 			spec:           []model.PortForward{{ContainerPort: 8000, LocalPort: 8080}},
// 			containerPorts: []int32{8000, 8080, 8001},
// 			expected:       []model.PortForward{{ContainerPort: 8000, LocalPort: 8080}},
// 		},
// 	}
//
// 	for i, c := range cases {
// 		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
// 			m := model.Manifest{Name: "fe"}.WithDeployTarget(model.K8sTarget{
// 				PortForwards: c.spec,
// 			})
// 			pod := v1alpha1.Pod{
// 				Containers: []v1alpha1.Container{
// 					v1alpha1.Container{Ports: c.containerPorts},
// 				},
// 			}
//
// 			actual := populatePortForwards(m, pod)
// 			assert.Equal(t, c.expected, actual)
// 		})
// 	}
// }

type pfsFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	cancel func()
	kCli   *k8s.FakeK8sClient
	st     *store.Store
	s      *Subscriber
	done   chan error
}

func newPFSFixture(t *testing.T) *pfsFixture {
	reducer := func(ctx context.Context, engineState *store.EngineState, action store.Action) {
		switch action := action.(type) {
		case PortForwardCreateAction:
			HandlePortForwardCreateAction(engineState, action)
		case PortForwardDeleteAction:
			HandlePortForwardDeleteAction(engineState, action)
		default:
			t.Fatalf("unrecognized action: %T", action)
		}
	}

	f := tempdir.NewTempDirFixture(t)
	st := store.NewStore(reducer, store.LogActionsFlag(false))
	kCli := k8s.NewFakeK8sClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	return &pfsFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		st:             st,
		kCli:           kCli,
		s:              NewSubscriber(kCli),
		done:           make(chan error),
	}
}

func (f *pfsFixture) onChange() {
	f.s.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
	time.Sleep(10 * time.Millisecond)
}
func (f pfsFixture) Start() {
	go func() {
		err := f.st.Loop(f.ctx)
		f.done <- err
	}()
}

func (f pfsFixture) WaitUntilDone() {
	err := <-f.done
	if err != nil && err != context.Canceled {
		f.T().Fatalf("Loop failed unexpectedly: %v", err)
	}
}

func (f *pfsFixture) assertForward(fwd Forward, expectedLocal, expectedContainer int32) {
	assert.Equal(f.T(), expectedLocal, fwd.LocalPort)
	assert.Equal(f.T(), expectedContainer, fwd.ContainerPort)
}

func (f *pfsFixture) assertOneForward(expectedLocal, expectedContainer int32, pf *PortForward) {
	if assert.Len(f.T(), pf.Spec.Forwards, 1) {
		assert.Equal(f.T(), expectedLocal, pf.Spec.Forwards[0].LocalPort)
		assert.Equal(f.T(), expectedContainer, pf.Spec.Forwards[0].ContainerPort)
	}
}

func (f *pfsFixture) waitUntilStatePortForwards(msg string, isDone func(map[string]*PortForward) bool) {
	f.T().Helper()

	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	isCanceled := false

	for {
		state := f.st.RLockState()
		done := isDone(state.PortForwards)
		fatalErr := state.FatalError
		f.st.RUnlockState()
		if done {
			return
		}
		if fatalErr != nil {
			f.T().Fatalf("Store had fatal error: %v", fatalErr)
		}

		if isCanceled {
			f.T().Fatalf("Timed out waiting for: %s", msg)
		}

		select {
		case <-ctx.Done():
			// Let the loop run the isDone test one more time
			isCanceled = true
		case <-time.Tick(10 * time.Millisecond):
		}
	}
}

func (f *pfsFixture) TearDown() {
	f.kCli.TearDown()
	f.TempDirFixture.TearDown()
	f.cancel()
	f.WaitUntilDone()
}
