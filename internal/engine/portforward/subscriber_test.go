package portforward

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestPortForwardNewPod(t *testing.T) {
	f := newPFSFixture(t)
	f.Start()
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}
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
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-A" && f.oneForwardMatches(8080, 8081, pf)
	})

	state = f.st.LockMutableStateForTesting()
	mt = state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-B", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("one port forward for pod B (replacing port forward for pod A)", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-B" && f.oneForwardMatches(8080, 8081, pf)
	})

	state = f.st.LockMutableStateForTesting()
	mt = state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-B", Phase: string(v1.PodPending)})
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("port forward for pod B has been torn down", func(pfs map[string]*PortForward) bool {
		return len(pfs) == 0
	})
}

func TestPortForwardChangePort(t *testing.T) {
	f := newPFSFixture(t)
	f.Start()
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8080,
				ContainerPort: 8081,
			},
		},
	})
	mt := store.NewManifestTarget(m)
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	state.UpsertManifestTarget(mt)
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("initial port forward", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-id" && f.oneForwardMatches(8080, 8081, pf)
	})

	state = f.st.LockMutableStateForTesting()
	kTarget := state.ManifestTargets["fe"].Manifest.K8sTarget()
	kTarget.PortForwards[0].ContainerPort = 8082
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("updated container port", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-id" && f.oneForwardMatches(8080, 8082, pf)
	})
}

func TestPortForwardChangeHost(t *testing.T) {
	f := newPFSFixture(t)
	f.Start()
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8080,
				ContainerPort: 8081,
				Host:          "hostA",
			},
		},
	})
	mt := store.NewManifestTarget(m)
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	state.UpsertManifestTarget(mt)
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("initial port forward", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-id" && f.oneForwardWithHostMatches(8080, 8081, "hostA", pf)
	})

	state = f.st.LockMutableStateForTesting()
	kTarget := state.ManifestTargets["fe"].Manifest.K8sTarget()
	kTarget.PortForwards[0].Host = "hostB"
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("updated host", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-id" && f.oneForwardWithHostMatches(8080, 8081, "hostB", pf)
	})
}

func TestPortForwardChangeManifestName(t *testing.T) {
	f := newPFSFixture(t)
	f.Start()
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8080,
				ContainerPort: 8081,
			},
		},
	})
	mt := store.NewManifestTarget(m)
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	state.UpsertManifestTarget(mt)
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("one port forward for manifest fe", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-id" && f.oneForwardMatches(8080, 8081, pf) &&
			pf.ObjectMeta.Annotations[v1alpha1.AnnotationManifest] == "fe"
	})

	state = f.st.LockMutableStateForTesting()
	// the exact same manifest, pod, etc., just with a different name
	mt = state.ManifestTargets["fe"]
	state.RemoveManifestTarget("fe")
	mt.Manifest.Name = "not-fe"
	state.UpsertManifestTarget(mt)
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("one port forward for manifest not-fe", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-id" && f.oneForwardMatches(8080, 8081, pf) &&
			pf.ObjectMeta.Annotations[v1alpha1.AnnotationManifest] == "not-fe"
	})
}

func TestMultipleForwardsOnePod(t *testing.T) {
	f := newPFSFixture(t)
	f.Start()
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{
				LocalPort:     8000,
				ContainerPort: 8080,
				Host:          "first-host",
			},
			{
				LocalPort:     9000,
				ContainerPort: 9090,
				Host:          "second-host",
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
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("one port forward with multiple Forwards", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}

		var seen8000, seen9000 bool
		pf := f.onlyPFFromMap(pfs)
		if pf.Spec.PodName != "pod-id" {
			return false
		}

		for _, fwd := range pf.Spec.Forwards {
			if fwd.LocalPort == 8000 {
				seen8000 = true
				f.forwardWithHostMatches(fwd, 8000, 8080, "first-host")
			} else if fwd.LocalPort == 9000 {
				seen9000 = true
				f.forwardWithHostMatches(fwd, 9000, 9090, "second-host")
			} else {
				t.Fatalf("found Forward to unexpected LocalPort: %+v", fwd)
			}
		}

		return seen8000 && seen9000
	})

	state = f.st.LockMutableStateForTesting()
	state.RemoveManifestTarget("fe")
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("port forward torn down", func(pfs map[string]*PortForward) bool {
		return len(pfs) == 0
	})
}

func TestPortForwardAutoDiscovery(t *testing.T) {
	f := newPFSFixture(t)
	f.Start()
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{{LocalPort: 8080}},
	})
	state.UpsertManifestTarget(store.NewManifestTarget(m))

	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest, v1alpha1.Pod{
		Name:  "pod-id",
		Phase: string(v1.PodRunning),
		Containers: []v1alpha1.Container{
			{Ports: []int32{8000}},
		},
	})
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStatePortForwards("running port forward with auto-discovered container port", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-id" && f.oneForwardMatches(8080, 8000, pf)
	})
}

func TestPortForwardAutoDiscovery2(t *testing.T) {
	f := newPFSFixture(t)
	f.Start()
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "fe"}
	m = m.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{{LocalPort: 8080}},
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
	f.waitUntilStatePortForwards("running port forward with auto-discovered container port", func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}
		pf := f.onlyPFFromMap(pfs)
		return pf.Spec.PodName == "pod-id" && f.oneForwardMatches(8080, 8080, pf)
	})
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

func (f *pfsFixture) forwardMatches(fwd Forward, expectedLocal, expectedContainer int32) bool {
	return expectedLocal == fwd.LocalPort && expectedContainer == fwd.ContainerPort
}

func (f *pfsFixture) oneForwardMatches(expectedLocal, expectedContainer int32, pf *PortForward) bool {
	return len(pf.Spec.Forwards) == 1 && f.forwardMatches(pf.Spec.Forwards[0], expectedLocal, expectedContainer)
}

func (f *pfsFixture) forwardWithHostMatches(fwd Forward, expectedLocal, expectedContainer int32, expectedHost string) bool {
	return expectedLocal == fwd.LocalPort &&
		expectedContainer == fwd.ContainerPort &&
		expectedHost == fwd.Host
}

func (f *pfsFixture) oneForwardWithHostMatches(expectedLocal, expectedContainer int32, expectedHost string, pf *PortForward) bool {
	return len(pf.Spec.Forwards) == 1 && f.forwardWithHostMatches(pf.Spec.Forwards[0], expectedLocal, expectedContainer, expectedHost)
}

func (f *pfsFixture) onlyPFFromMap(pfs map[string]*PortForward) *PortForward {
	require.Len(f.T(), pfs, 1, "`onlyPFFromMap` requires a map of length one (got %d)", len(pfs))
	for _, pf := range pfs { // there's only one thing in the map, we just don't know its key 🙃
		return pf
	}
	return nil
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
