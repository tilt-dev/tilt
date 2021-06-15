package portforward

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/controllers/fake"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

type expectedPF struct {
	podID     string
	local     int32
	container int32
	host      string
	mName     string
}

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
	f.waitUntilNoStateOrAPIPortForwards("no port forwards running yet")

	state = f.st.LockMutableStateForTesting()
	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-A", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStateAndAPIPortForwardAsExpected("one port forward for pod A", expectedPF{podID: "pod-A", local: 8080, container: 8081})

	state = f.st.LockMutableStateForTesting()
	mt = state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-B", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilPortForwardDNEOnStateOrAPI("port forward for pod A has been removed", "pod-A")
	f.waitUntilStateAndAPIPortForwardAsExpected("new port forward for pod B", expectedPF{podID: "pod-B", local: 8080, container: 8081})

	state = f.st.LockMutableStateForTesting()
	mt = state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-B", Phase: string(v1.PodPending)})
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilNoStateOrAPIPortForwards("port forward for pod B has been torn down")
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
	f.waitUntilStateAndAPIPortForwardAsExpected("initial port forward", expectedPF{podID: "pod-id", local: 8080, container: 8081})

	state = f.st.LockMutableStateForTesting()
	kTarget := state.ManifestTargets["fe"].Manifest.K8sTarget()
	kTarget.PortForwards[0].ContainerPort = 8082
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStateAndAPIPortForwardAsExpected("updated container port", expectedPF{podID: "pod-id", local: 8080, container: 8082})
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
	f.waitUntilStateAndAPIPortForwardAsExpected("initial port forward", expectedPF{podID: "pod-id", local: 8080, container: 8081, host: "hostA"})

	state = f.st.LockMutableStateForTesting()
	kTarget := state.ManifestTargets["fe"].Manifest.K8sTarget()
	kTarget.PortForwards[0].Host = "hostB"
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStateAndAPIPortForwardAsExpected("updated host", expectedPF{podID: "pod-id", local: 8080, container: 8081, host: "hostB"})
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
	f.waitUntilStateAndAPIPortForwardAsExpected("one port forward for manifest fe", expectedPF{podID: "pod-id", local: 8080, container: 8081, mName: "fe"})

	state = f.st.LockMutableStateForTesting()
	// the exact same manifest, pod, etc., just with a different name
	mt = state.ManifestTargets["fe"]
	state.RemoveManifestTarget("fe")
	mt.Manifest.Name = "not-fe"
	state.UpsertManifestTarget(mt)
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStateAndAPIPortForwardAsExpected("manifest name has been updated", expectedPF{podID: "pod-id", local: 8080, container: 8081, mName: "not-fe"})
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
	f.waitUntilNoStateOrAPIPortForwards("no port forwards running yet")

	state = f.st.LockMutableStateForTesting()
	mt := state.ManifestTargets["fe"]
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(mt.Manifest,
		v1alpha1.Pod{Name: "pod-id", Phase: string(v1.PodRunning)})
	f.st.UnlockMutableState()

	f.onChange()
	f.waitUntilStateAndAPIPortForward("one port forward with multiple Forwards", pfName("pod-id"), func(pf *PortForward) bool {
		var seen8000, seen9000 bool
		if pf.Spec.PodName != "pod-id" {
			return false
		}

		for _, fwd := range pf.Spec.Forwards {
			if fwd.LocalPort == 8000 {
				seen8000 = true
				f.forwardMatches(fwd, expectedPF{local: 8000, container: 8080, host: "first-host"})
			} else if fwd.LocalPort == 9000 {
				seen9000 = true
				f.forwardMatches(fwd, expectedPF{local: 9000, container: 9090, host: "second-host"})
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
	f.waitUntilNoStateOrAPIPortForwards("port forward torn down")
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
	f.waitUntilStateAndAPIPortForwardAsExpected("running port forward with auto-discovered container port", expectedPF{podID: "pod-id", local: 8080, container: 8000})
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
	f.waitUntilStateAndAPIPortForwardAsExpected("running port forward with auto-discovered container port", expectedPF{podID: "pod-id", local: 8080, container: 8080})
}

func TestPopulatePortForward(t *testing.T) {
	cases := []struct {
		spec           []model.PortForward
		containerPorts []int32
		expected       []model.PortForward
	}{
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
	ctrl   *fake.ControllerFixture
	st     *store.Store
	s      *Subscriber
	done   chan error
}

func newPFSFixture(t *testing.T) *pfsFixture {
	reducer := func(ctx context.Context, engineState *store.EngineState, action store.Action) {
		t.Helper()
		switch action := action.(type) {
		case store.ErrorAction:
			t.Fatalf("reducer received unexpected ErrorAction: %+v", action.Error)
			return
		case PortForwardUpsertAction:
			HandlePortForwardUpsertAction(engineState, action)
		case PortForwardDeleteAction:
			HandlePortForwardDeleteAction(engineState, action)
		default:
			t.Fatalf("unrecognized action (%T): %+v", action, action)
			return
		}
	}

	f := tempdir.NewTempDirFixture(t)
	st := store.NewStore(reducer, store.LogActionsFlag(false))
	kCli := k8s.NewFakeK8sClient(t)

	// only testing object create/delete, not reconciliation, so pass a nil reconciler
	ctrl := fake.NewControllerFixture(t, nil)

	ctx, cancel := context.WithCancel(context.Background())

	return &pfsFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		st:             st,
		ctrl:           ctrl,
		kCli:           kCli,
		s:              NewSubscriber(kCli, ctrl.Client),
		done:           make(chan error),
	}
}

func (f *pfsFixture) onChange() {
	_ = f.s.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
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

func (f *pfsFixture) forwardMatches(fwd Forward, expected expectedPF) bool {
	if expected.local != 0 && expected.local != fwd.LocalPort {
		return false
	}
	if expected.container != 0 && expected.container != fwd.ContainerPort {
		return false
	}
	if expected.host != "" && expected.host != fwd.Host {
		return false
	}
	return true
}

func (f *pfsFixture) oneForwardMatches(pf *PortForward, expected expectedPF) bool {
	if len(pf.Spec.Forwards) != 1 {
		return false
	}

	if !f.forwardMatches(pf.Spec.Forwards[0], expected) {
		return false
	}
	return expected.mName == "" || pf.ObjectMeta.Annotations[v1alpha1.AnnotationManifest] == expected.mName
}

// Use this func to assert on an expected PortForward with a single Fwd.
// For more complicated assertions, use waitUntilStateAndAPIPortForward directly.
func (f *pfsFixture) waitUntilStateAndAPIPortForwardAsExpected(msg string, expected expectedPF) {
	f.T().Helper()

	if expected.podID == "" {
		f.T().Fatal("must pass pod ID as part of expectedPF")
	}

	f.waitUntilStateAndAPIPortForward(msg, pfName(expected.podID), func(pf *PortForward) bool {
		return f.oneForwardMatches(pf, expected)
	})
}

func (f *pfsFixture) waitUntilStateAndAPIPortForward(msg string, name string, pfOK func(pf *PortForward) bool) {
	f.T().Helper()
	f.waitUntilStatePortForwards(msg, func(pfs map[string]*PortForward) bool {
		if len(pfs) != 1 {
			return false
		}

		pf, ok := pfs[name]
		if !ok {
			return false
		}

		return pfOK(pf)
	})

	key := types.NamespacedName{Name: name}
	f.requireState(key, func(pf *PortForward) bool {
		if pf == nil {
			return false
		}

		return pfOK(pf)

	}, "Expected port forward API object not observed for key %s", key)
}

func (f *pfsFixture) waitUntilNoStateOrAPIPortForwards(msg string) {
	// State PFs
	f.waitUntilStatePortForwards(msg, func(pfs map[string]*PortForward) bool {
		return len(pfs) == 0
	})

	// API PFs
	var foundPFs v1alpha1.PortForwardList
	require.Eventuallyf(f.T(), func() bool {
		var pfs v1alpha1.PortForwardList
		f.ctrl.List(&pfs)
		foundPFs = pfs
		return len(pfs.Items) == 0
	}, time.Second, 20*time.Millisecond, "Expected no port forward API objects to exist, but found %d: %+v", len(foundPFs.Items), foundPFs.Items)
}

func (f *pfsFixture) waitUntilPortForwardDNEOnStateOrAPI(msg string, podID string) {
	name := pfName(podID)
	// State PFs
	f.waitUntilStatePortForwards(msg, func(pfs map[string]*PortForward) bool {
		_, ok := pfs[name]
		return !ok
	})

	// API PFs
	var foundPF *PortForward
	key := types.NamespacedName{Name: name}
	f.requireState(key, func(pf *PortForward) bool {
		foundPF = pf
		return pf == nil
	}, "Expected port forward API object %s to not exist, but found: %+v", name, foundPF)
}

func (f *pfsFixture) requireState(key types.NamespacedName, cond func(pf *PortForward) bool, msg string, args ...interface{}) {
	f.T().Helper()
	require.Eventuallyf(f.T(), func() bool {
		var pf PortForward
		if !f.ctrl.Get(key, &pf) {
			return cond(nil)
		}
		return cond(&pf)
	}, time.Second, 20*time.Millisecond, msg, args...)
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
