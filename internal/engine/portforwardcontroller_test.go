package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
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

	f.plc.OnChange(f.ctx, f.st)
	assert.Equal(t, 0, len(f.plc.activeForwards))

	state = f.st.LockMutableStateForTesting()
	state.ManifestTargets["fe"].State.PodSet = store.NewPodSet(store.Pod{PodID: "pod-id", Phase: v1.PodRunning})
	f.st.UnlockMutableState()

	f.plc.OnChange(f.ctx, f.st)
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, "pod-id", f.kCli.LastForwardPortPodID.String())

	state = f.st.LockMutableStateForTesting()
	state.ManifestTargets["fe"].State.PodSet = store.NewPodSet(store.Pod{PodID: "pod-id2", Phase: v1.PodRunning})
	f.st.UnlockMutableState()

	f.plc.OnChange(f.ctx, f.st)
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, "pod-id2", f.kCli.LastForwardPortPodID.String())

	state = f.st.LockMutableStateForTesting()
	state.ManifestTargets["fe"].State.PodSet = store.NewPodSet(store.Pod{PodID: "pod-id2", Phase: v1.PodPending})
	f.st.UnlockMutableState()

	f.plc.OnChange(f.ctx, f.st)
	assert.Equal(t, 0, len(f.plc.activeForwards))
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
	state.ManifestTargets["fe"].State.PodSet = store.NewPodSet(store.Pod{PodID: "pod-id", Phase: v1.PodRunning})
	f.st.UnlockMutableState()

	f.plc.OnChange(f.ctx, f.st)
	assert.Equal(t, 0, len(f.plc.activeForwards))

	state = f.st.LockMutableStateForTesting()
	state.ManifestTargets["fe"].State.PodSet.Pods["pod-id"].Containers = []store.Container{
		store.Container{Ports: []int32{8000}},
	}
	f.st.UnlockMutableState()

	f.plc.OnChange(f.ctx, f.st)
	assert.Equal(t, 1, len(f.plc.activeForwards))
	assert.Equal(t, 8000, f.kCli.LastForwardPortRemotePort)
}

type plcFixture struct {
	*tempdir.TempDirFixture
	ctx  context.Context
	kCli *k8s.FakeK8sClient
	st   *store.Store
	plc  *PortForwardController
}

func newPLCFixture(t *testing.T) *plcFixture {
	f := tempdir.NewTempDirFixture(t)
	st, _ := store.NewStoreForTesting()
	kCli := k8s.NewFakeK8sClient()
	plc := NewPortForwardController(kCli)
	return &plcFixture{
		TempDirFixture: f,
		ctx:            context.Background(),
		st:             st,
		kCli:           kCli,
		plc:            plc,
	}
}

func (f *plcFixture) TearDown() {
	f.kCli.TearDown()
	f.TempDirFixture.TearDown()
}
