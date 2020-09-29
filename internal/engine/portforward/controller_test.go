package portforward

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestPortForward(t *testing.T) {
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
		store.Pod{PodID: "pod-id", Phase: v1.PodRunning})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, "pod-id", f.kCli.LastForwardPortPodID.String())
	firstPodForwardCtx := f.kCli.LastForwardContext

	state = f.st.LockMutableStateForTesting()
	mt = state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		store.Pod{PodID: "pod-id2", Phase: v1.PodRunning})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, "pod-id2", f.kCli.LastForwardPortPodID.String())

	state = f.st.LockMutableStateForTesting()
	mt = state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, store.Pod{PodID: "pod-id2", Phase: v1.PodPending})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 0, len(f.plc.activeForwards))

	assert.Equal(t, context.Canceled, firstPodForwardCtx.Err(),
		"Expected first port-forward to be canceled")
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
		store.Pod{PodID: "pod-id", Phase: v1.PodRunning})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 0, len(f.plc.activeForwards))
	state = f.st.LockMutableStateForTesting()
	state.ManifestTargets["fe"].State.K8sRuntimeState().Pods["pod-id"].Containers = []store.Container{
		store.Container{Ports: []int32{8000}},
	}
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8000, f.kCli.LastForwardPortRemotePort)
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
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, store.Pod{
		PodID: "pod-id",
		Phase: v1.PodRunning,
		Containers: []store.Container{
			store.Container{Ports: []int32{8000, 8080}},
		},
	})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8080, f.kCli.LastForwardPortRemotePort)
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
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, store.Pod{PodID: "pod-id", Phase: v1.PodRunning})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8081, f.kCli.LastForwardPortRemotePort)

	state = f.st.LockMutableStateForTesting()
	kTarget := state.ManifestTargets["fe"].Manifest.K8sTarget()
	kTarget.PortForwards[0].ContainerPort = 8082
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8082, f.kCli.LastForwardPortRemotePort)
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
		store.Pod{PodID: "pod-id", Phase: v1.PodRunning})
	f.st.UnlockMutableState()

	f.onChange()
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 1, f.kCli.CreatePortForwardCallCount)

	err := fmt.Errorf("unique-error")
	f.kCli.LastForwarder.Done <- err

	assert.Contains(t, "unique-error", f.out.String())
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 2, f.kCli.CreatePortForwardCallCount)
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
			pod := store.Pod{
				Containers: []store.Container{
					store.Container{Ports: c.containerPorts},
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
	plc    *Controller
	out    *bufsync.ThreadSafeBuffer
}

func newPLCFixture(t *testing.T) *plcFixture {
	f := tempdir.NewTempDirFixture(t)
	st := store.NewTestingStore()
	kCli := k8s.NewFakeK8sClient()
	plc := NewController(kCli)

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
	f.plc.OnChange(f.ctx, f.st)
	time.Sleep(10 * time.Millisecond)
}

func (f *plcFixture) TearDown() {
	f.kCli.TearDown()
	f.TempDirFixture.TearDown()
	f.cancel()
}
