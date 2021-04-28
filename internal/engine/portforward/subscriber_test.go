package portforward

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestPortForwardAndCreateDeleteActions(t *testing.T) {
	f := newPLCFixture(t)
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
	assert.Equal(t, 0, len(f.plc.activeForwards))

	state = f.st.LockMutableStateForTesting()
	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, "pod-id", f.kCli.LastForwardPortPodID().String())
	firstPodForwardCtx := f.kCli.LastForwardContext()
	f.assertPortForwardCreateAction(t, 8080, 8081, "pod-id")
	f.st.ClearActions()

	state = f.st.LockMutableStateForTesting()
	mt = state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id2", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, "pod-id2", f.kCli.LastForwardPortPodID().String())
	f.assertPortForwardDeleteAction(t)
	f.assertPortForwardCreateAction(t, 8080, 8081, "pod-id2")
	f.st.ClearActions()

	state = f.st.LockMutableStateForTesting()
	mt = state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id2", Phase: string(v1.PodPending)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 0, len(f.plc.activeForwards))
	f.assertPortForwardDeleteAction(t)
	f.st.ClearActions()

	assert.Equal(t, context.Canceled, firstPodForwardCtx.Err(),
		"Expected first port-forward to be canceled")
}

func TestMultiplePortForwards(t *testing.T) {
	f := newPLCFixture(t)
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{
		Name: "fe",
	}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8000,
				ContainerPort: 8080,
			},
			{
				LocalPort:     8001,
				ContainerPort: 8081,
			},
		},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 0, len(f.plc.activeForwards))

	state = f.st.LockMutableStateForTesting()
	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	require.Equal(t, 2, len(f.plc.activeForwards))
	require.Equal(t, 2, len(f.kCli.PortForwardCalls))

	// PortForwards are executed async so we can't guarantee the order;
	// just make sure each expected call appears exactly once
	expectedRemotePorts := []int{8080, 8081}
	var actualRemotePorts []int
	var contexts []context.Context
	for _, call := range f.kCli.PortForwardCalls {
		actualRemotePorts = append(actualRemotePorts, call.RemotePort)
		contexts = append(contexts, call.Context)
		assert.Equal(t, "pod-id", call.PodID.String())
	}
	assert.ElementsMatch(t, expectedRemotePorts, actualRemotePorts, "remote ports for which PortForward was called")

	state = f.st.LockMutableStateForTesting()
	mt = state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodPending)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 0, len(f.plc.activeForwards))

	for _, ctx := range contexts {
		assert.Equal(t, context.Canceled, ctx.Err(),
			"found uncancelled port forward context")
	}
}

func TestPortForwardAutoDiscovery(t *testing.T) {
	f := newPLCFixture(t)
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{
		Name: "fe",
	}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort: 8080,
			},
		},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))

	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 0, len(f.plc.activeForwards))
	state = f.st.LockMutableStateForTesting()
	state.ManifestTargets["fe"].State.K8sRuntimeState().Pods["pod-id"].Containers = []v1alpha1.Container{
		v1alpha1.Container{Ports: []int32{8000}},
	}
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8000, f.kCli.LastForwardPortRemotePort())
}

func TestPortForwardAutoDiscovery2(t *testing.T) {
	f := newPLCFixture(t)
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{
		Name: "fe",
	}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort: 8080,
			},
		},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))

	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, v1alpha1.Pod{
		Name:  "pod-id",
		Phase: string(v1.PodRunning),
		Containers: []v1alpha1.Container{
			{Ports: []int32{8000, 8080}},
		},
	})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8080, f.kCli.LastForwardPortRemotePort())
}

func TestPortForwardChangePort(t *testing.T) {
	f := newPLCFixture(t)
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8080,
				ContainerPort: 8081,
			},
		},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())

	state = f.st.LockMutableStateForTesting()
	kTarget := state.ManifestTargets["fe"].Manifest.K8sTarget()
	kTarget.PortForwards[0].ContainerPort = 8082
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8082, f.kCli.LastForwardPortRemotePort())
}

func TestPortForwardChangeHost(t *testing.T) {
	f := newPLCFixture(t)
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8080,
				ContainerPort: 8081,
				Host:          "someHostA",
			},
		},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())
	assert.Equal(t, "someHostA", f.kCli.LastForwardPortHost())

	state = f.st.LockMutableStateForTesting()
	kTarget := state.ManifestTargets["fe"].Manifest.K8sTarget()
	kTarget.PortForwards[0].Host = "otherHostB"
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())
	assert.Equal(t, "otherHostB", f.kCli.LastForwardPortHost())
}

func TestPortForwardChangeManifestName(t *testing.T) {
	f := newPLCFixture(t)
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "manifestA"}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8080,
				ContainerPort: 8081,
			},
		},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	mt := state.ManifestTargets["manifestA"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())
	for _, pf := range f.plc.activeForwards {
		assert.Equal(t, "manifestA", pf.ObjectMeta.Annotations[v1alpha1.AnnotationManifest], "port forward expected manifest name")
	}

	state = f.st.LockMutableStateForTesting()
	delete(state.ManifestTargets, "manifestA")
	m = model.Manifest{Name: "manifestB"}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8080,
				ContainerPort: 8081,
			},
		},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	mt = state.ManifestTargets["manifestB"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort())
	for _, pf := range f.plc.activeForwards {
		assert.Equal(t, "manifestB", pf.ObjectMeta.Annotations[v1alpha1.AnnotationManifest], "port forward expected manifest name")
	}
}

func TestPortForwardRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): investigate")
	}
	f := newPLCFixture(t)
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8080,
				ContainerPort: 8081,
			},
		},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 1, f.kCli.CreatePortForwardCallCount())

	err := fmt.Errorf("unique-error")
	f.kCli.LastForwarder().Done <- err

	assert.Contains(t, "unique-error", f.out.String())
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 2, f.kCli.CreatePortForwardCallCount())
}

type portForwardTestCase struct {
	spec           []model.PortForward
	containerPorts []int32
	expected       []model.PortForward
}

func TestPopulatePortForward(t *testing.T) {
	cases := []portForwardTestCase{
		{
			spec:           []model.PortForward{{LocalPort: 8080}},
			containerPorts: []int32{8080},
			expected:       []model.PortForward{{ContainerPort: 8080, LocalPort: 8080}},
		},
		{
			spec:           []model.PortForward{{LocalPort: 8080}},
			containerPorts: []int32{8000, 8080, 8001},
			expected:       []model.PortForward{{ContainerPort: 8080, LocalPort: 8080}},
		},
		{
			spec:           []model.PortForward{{LocalPort: 8080}, {LocalPort: 8000}},
			containerPorts: []int32{8000, 8080, 8001},
			expected: []model.PortForward{
				{ContainerPort: 8080, LocalPort: 8080},
				{ContainerPort: 8000, LocalPort: 8000},
			},
		},
		{
			spec:           []model.PortForward{{ContainerPort: 8000, LocalPort: 8080}},
			containerPorts: []int32{8000, 8080, 8001},
			expected:       []model.PortForward{{ContainerPort: 8000, LocalPort: 8080}},
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			m := model.Manifest{Name: "fe"}.WithDeployTarget(model.K8sTarget{
				PortForwards: c.spec,
			})
			pod := v1alpha1.Pod{
				Containers: []v1alpha1.Container{
					v1alpha1.Container{Ports: c.containerPorts},
				},
			}

			actual := populatePortForwards(m, pod)
			assert.Equal(t, c.expected, actual)
		})
	}
}

type plcFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	cancel func()
	kCli   *k8s.FakeK8sClient
	st     *store.TestingStore
	plc    *Subscriber
	out    *bufsync.ThreadSafeBuffer
}

func newPLCFixture(t *testing.T) *plcFixture {
	f := tempdir.NewTempDirFixture(t)
	st := store.NewTestingStore()
	kCli := k8s.NewFakeK8sClient()
	plc := NewSubscriber(kCli)

	out := bufsync.NewThreadSafeBuffer()
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.WithLogger(ctx, l)
	return &plcFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		st:             st,
		kCli:           kCli,
		plc:            plc,
		out:            out,
	}
}

func (f *plcFixture) onChange() {
	f.plc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
	time.Sleep(10 * time.Millisecond)
}

func (f *plcFixture) assertPortForwardCreateAction(t *testing.T, localPort, containerPort int, podName string) {
	t.Helper()

	a := f.st.WaitForAction(t, reflect.TypeOf(PortForwardCreateAction{}))
	pf := a.(PortForwardCreateAction).PortForward
	assert.Equal(t, localPort, pf.Spec.LocalPort, "local port from PortForwardCreateAction")
	assert.Equal(t, containerPort, pf.Spec.ContainerPort, "container port from PortForwardCreateAction")
	assert.Equal(t, podName, pf.Spec.PodName, "pod name from PortForwardCreateAction")

	f.st.ClearActions()
}

func (f *plcFixture) assertPortForwardDeleteAction(t *testing.T) {
	t.Helper()

	// It's a PITA to reconstruct the name we expect to see on this action,
	// so for now just assert that an action appeared
	f.st.WaitForAction(t, reflect.TypeOf(PortForwardDeleteAction{}))
}

func (f *plcFixture) TearDown() {
	f.kCli.TearDown()
	f.TempDirFixture.TearDown()
	f.cancel()
}
