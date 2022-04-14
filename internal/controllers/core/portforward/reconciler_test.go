package portforward

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/pkg/apis"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
)

const (
	pfFooName = "pf_foo"
	pfBarName = "pf_bar"
)

func TestCreatePortForward(t *testing.T) {
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	pf := f.makeSimplePF(pfFooName, 8000, 8080)
	f.Create(pf)
	kCli := f.clients.MustK8sClient(clusterNN(pf))

	f.requirePortForwardStarted(pfFooName, 8000, 8080)
	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, kCli.LastForwardPortRemotePort())
}

func TestDeletePortForward(t *testing.T) {
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	pf := f.makeSimplePF(pfFooName, 8000, 8080)
	f.Create(pf)
	kCli := f.clients.MustK8sClient(clusterNN(pf))
	f.requirePortForwardStarted(pfFooName, 8000, 8080)

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, kCli.LastForwardPortRemotePort())
	origForwardCtx := kCli.LastForwardContext()

	f.Delete(pf)
	f.requirePortForwardDeleted(pfFooName)

	require.Equal(t, 0, len(f.r.activeForwards))
	f.assertContextCancelled(t, origForwardCtx)
}

func TestModifyPortForward(t *testing.T) {
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	pf := f.makeSimplePF(pfFooName, 8000, 8080)
	f.Create(pf)
	kCli := f.clients.MustK8sClient(clusterNN(pf))
	f.requirePortForwardStarted(pfFooName, 8000, 8080)

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, kCli.LastForwardPortRemotePort())
	origForwardCtx := kCli.LastForwardContext()

	pf = f.makeSimplePF(pfFooName, 8001, 9090)
	f.GetAndUpdate(pf)
	f.requirePortForwardStarted(pfFooName, 8001, 9090)

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", kCli.LastForwardPortPodID().String())
	assert.Equal(t, 9090, kCli.LastForwardPortRemotePort())

	f.assertContextCancelled(t, origForwardCtx)
}

func TestModifyPortForwardManifestName(t *testing.T) {
	// A change to only the manifestName should be enough to tear down and recreate
	// a PortForward (we need to do this so the logs will be routed correctly)
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	fwds := []v1alpha1.Forward{f.makeForward(8000, 8080, "")}

	pf := f.makePF(pfFooName, "manifestA", "pod-pf_foo", "", fwds)
	f.Create(pf)
	kCli := f.clients.MustK8sClient(clusterNN(pf))
	f.requirePortForwardStarted(pfFooName, 8000, 8080)

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, kCli.LastForwardPortRemotePort())
	origForwardCtx := kCli.LastForwardContext()

	pf = f.makePF(pfFooName, "manifestB", "pod-pf_foo", "", fwds)
	f.GetAndUpdate(pf)
	f.requireState(pfFooName, func(pf *PortForward) bool {
		return pf != nil && pf.ObjectMeta.Annotations[v1alpha1.AnnotationManifest] == "manifestB"
	}, "Manifest annotation was not updated")
	f.requirePortForwardStarted(pfFooName, 8000, 8080)

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, kCli.LastForwardPortRemotePort())

	f.assertContextCancelled(t, origForwardCtx)
}

func TestMultipleForwardsForOnePod(t *testing.T) {
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	forwards := []v1alpha1.Forward{
		f.makeForward(8000, 8080, "hostA"),
		f.makeForward(8001, 8081, "hostB"),
	}

	pf := f.makeSimplePFMultipleForwards(pfFooName, forwards)
	f.Create(pf)
	kCli := f.clients.MustK8sClient(clusterNN(pf))
	f.requirePortForwardStarted(pfFooName, 8000, 8080)
	f.requirePortForwardStarted(pfFooName, 8001, 8081)

	require.Equal(t, 1, len(f.r.activeForwards))
	require.Equal(t, 2, kCli.CreatePortForwardCallCount())

	var seen8080, seen8081 bool
	var contexts []context.Context
	for _, call := range kCli.PortForwardCalls() {
		assert.Equal(t, "pod-pf_foo", call.PodID.String())
		switch call.RemotePort {
		case 8080:
			seen8080 = true
			contexts = append(contexts, call.Context)
			assert.Equal(t, "hostA", call.Host, "unexpected host for port forward to 8080")
		case 8081:
			seen8081 = true
			contexts = append(contexts, call.Context)
			assert.Equal(t, "hostB", call.Host, "unexpected host for port forward to 8081")
		default:
			t.Fatalf("found port forward call to unexpected remotePort: %+v", call)
		}
	}
	require.True(t, seen8080, "did not see port forward to remotePort 8080")
	require.True(t, seen8081, "did not see port forward to remotePort 8081")

	f.Delete(pf)
	f.requirePortForwardDeleted(pfFooName)

	require.Equal(t, 0, len(f.r.activeForwards))
	for _, ctx := range contexts {
		f.assertContextCancelled(t, ctx)
	}
}

func TestMultipleForwardsMultiplePods(t *testing.T) {
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	fwdsFoo := []v1alpha1.Forward{f.makeForward(8000, 8080, "host-foo")}
	fwdsBar := []v1alpha1.Forward{f.makeForward(8001, 8081, "host-bar")}
	pfFoo := f.makePF(pfFooName, "foo", "pod-pf_foo", "ns-foo", fwdsFoo)
	pfBar := f.makePF(pfBarName, "bar", "pod-pf_bar", "ns-bar", fwdsBar)
	f.Create(pfFoo)
	f.Create(pfBar)
	kCli := f.clients.MustK8sClient(clusterNN(pfFoo))
	f.requirePortForwardStarted(pfFooName, 8000, 8080)
	f.requirePortForwardStarted(pfBarName, 8001, 8081)

	require.Equal(t, 2, len(f.r.activeForwards))
	require.Equal(t, 2, kCli.CreatePortForwardCallCount())

	// PortForwards are executed async so we can't guarantee the order;
	// just make sure each expected call appears exactly once
	var seenFoo, seenBar bool
	var ctxFoo, ctxBar context.Context
	for _, call := range kCli.PortForwardCalls() {
		if call.PodID.String() == "pod-pf_foo" {
			seenFoo = true
			ctxFoo = call.Context
			assert.Equal(t, 8080, call.RemotePort, "remotePort for forward foo")
			assert.Equal(t, "ns-foo", call.Forwarder.Namespace().String(), "namespace for forward foo")
			assert.Equal(t, "host-foo", call.Host, "host for forward foo")
		} else if call.PodID.String() == "pod-pf_bar" {
			seenBar = true
			ctxBar = call.Context
			assert.Equal(t, 8081, call.RemotePort, "remotePort for forward bar")
			assert.Equal(t, "ns-bar", call.Forwarder.Namespace().String(), "namespace for forward bar")
			assert.Equal(t, "host-bar", call.Host, "host for forward bar")
		} else {
			t.Fatalf("found port forward call for unexpected pod: %+v", call)
		}
	}
	require.True(t, seenFoo, "did not see port forward foo")
	require.True(t, seenBar, "did not see port forward bar")

	f.Delete(pfFoo)
	f.requirePortForwardDeleted(pfFooName)

	require.Equal(t, 1, len(f.r.activeForwards))
	f.assertContextCancelled(t, ctxFoo)
	f.assertContextNotCancelled(t, ctxBar)
}

func TestPortForwardStartFailure(t *testing.T) {
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	pf := f.makeSimplePF(pfFooName, k8s.MagicTestExplodingPort, 8080)
	f.Create(pf)

	f.requirePortForwardError(pfFooName, k8s.MagicTestExplodingPort, 8080,
		"fake error starting port forwarding")
}

func TestPortForwardRuntimeFailure(t *testing.T) {
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	pf := f.makeSimplePF(pfFooName, 8000, 8080)
	f.Create(pf)
	// wait for port forward to be successful
	f.requirePortForwardStarted(pfFooName, 8000, 8080)

	kCli := f.clients.MustK8sClient(clusterNN(pf))
	const errMsg = "fake runtime port forwarding error"
	kCli.LastForwarder().TriggerFailure(errors.New(errMsg))

	f.requirePortForwardError(pfFooName, 8000, 8080, errMsg)
}

func TestPortForwardPartialSuccess(t *testing.T) {
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	forwards := []Forward{
		f.makeForward(8000, 8080, "localhost"),
		f.makeForward(8001, 8081, "localhost"),
		f.makeForward(k8s.MagicTestExplodingPort, 8082, "localhost"),
	}

	pf := f.makeSimplePFMultipleForwards(pfFooName, forwards)
	f.Create(pf)
	f.requirePortForwardStarted(pfFooName, 8000, 8080)
	f.requirePortForwardStarted(pfFooName, 8001, 8081)
	f.requirePortForwardError(pfFooName, k8s.MagicTestExplodingPort, 8082, "fake error starting port forwarding")

	kCli := f.clients.MustK8sClient(clusterNN(pf))
	const errMsg = "fake runtime port forwarding error"
	for _, pfCall := range kCli.PortForwardCalls() {
		if pfCall.RemotePort == 8080 {
			pfCall.Forwarder.TriggerFailure(errors.New(errMsg))
		}
	}

	f.requirePortForwardError(pfFooName, 8000, 8080, errMsg)
	f.requirePortForwardStarted(pfFooName, 8001, 8081)
	f.requirePortForwardError(pfFooName, k8s.MagicTestExplodingPort, 8082, "fake error starting port forwarding")
}

func TestIndexing(t *testing.T) {
	f := newPFRFixture(t)

	pf := f.makeSimplePF(pfFooName, 8000, 8080)
	f.Create(pf)
	f.MustGet(apis.Key(pf), pf)

	reqs := f.r.indexer.Enqueue(&v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: pfFooName}},
	}, reqs)

	reqs = f.r.indexer.Enqueue(&v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "other"}})
	require.Empty(t, reqs)
}

func TestClusterChange(t *testing.T) {
	f := newPFRFixture(t)

	require.Equal(t, 0, len(f.r.activeForwards))

	pf := f.makeSimplePF(pfFooName, 8000, 8080)
	f.Create(pf)
	clusterKey := clusterNN(pf)

	// port forward should be started and have made a call to fake client
	f.requirePortForwardStarted(pfFooName, 8000, 8080)
	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo",
		f.clients.MustK8sClient(clusterKey).LastForwardPortPodID().String())

	// put the cluster into an error state and verify that active forward(s)
	// are stopped
	f.clients.EnsureK8sClusterError(f.Context(), clusterKey, errors.New("oh no"))
	_, err := f.Reconcile(apis.Key(pf))
	require.EqualError(t, err, "oh no")
	require.Empty(t, len(f.r.activeForwards),
		"Port forward should have been stopped")

	// create a new healthy client and verify that it gets used
	kCli, _ := f.clients.EnsureK8sCluster(f.Context(), clusterKey)
	require.Zero(t, kCli.CreatePortForwardCallCount(),
		"No port forwards should exist")
	f.MustReconcile(apis.Key(pf))
	f.requirePortForwardStarted(pfFooName, 8000, 8080)
	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, kCli.LastForwardPortRemotePort())
}

type pfrFixture struct {
	*fake.ControllerFixture
	t       *testing.T
	st      store.RStore
	r       *Reconciler
	clients *cluster.FakeClientProvider
}

func newPFRFixture(t *testing.T) *pfrFixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	clients := cluster.NewFakeClientProvider(t, cfb.Client)
	r := NewReconciler(cfb.Client, cfb.Scheme(), cfb.Store, clients)
	indexer.StartSourceForTesting(cfb.Context(), r.requeuer, r, nil)

	return &pfrFixture{
		ControllerFixture: cfb.Build(r),
		t:                 t,
		st:                cfb.Store,
		r:                 r,
		clients:           clients,
	}
}

func (f *pfrFixture) requireState(name string, cond func(pf *PortForward) bool, msg string, args ...interface{}) {
	f.t.Helper()
	key := types.NamespacedName{Name: name}
	require.Eventuallyf(f.t, func() bool {
		var pf PortForward
		if !f.Get(key, &pf) {
			return cond(nil)
		}
		return cond(&pf)
	}, 2*time.Second, 20*time.Millisecond, msg, args...)
}

func (f *pfrFixture) requirePortForwardStatus(name string, localPort, containerPort int32, cond func(ForwardStatus) (bool, string)) {
	f.t.Helper()
	var desc strings.Builder
	f.requireState(name, func(pf *PortForward) bool {
		desc.Reset()
		if pf == nil {
			desc.WriteString("object does not exist in api")
			return false
		}
		for _, f := range pf.Status.ForwardStatuses {
			if f.LocalPort != localPort || f.ContainerPort != containerPort {
				continue
			}
			ok, msg := cond(*f.DeepCopy())
			desc.WriteString(msg)
			return ok
		}
		desc.WriteString("did not find matching forward status for ports:\n")
		desc.WriteString(spew.Sdump(pf.Status.ForwardStatuses))
		return false
	}, "PortForward %q status for localPort=%d / containerPort=%d did not match condition: %s", name, localPort, containerPort, &desc)
}

func (f *pfrFixture) requirePortForwardError(name string, localPort, containerPort int32, errMsg string) {
	f.t.Helper()
	f.requirePortForwardStatus(name, localPort, containerPort, func(status ForwardStatus) (bool, string) {
		if !strings.Contains(status.Error, errMsg) {
			return false, fmt.Sprintf("error %q does not contain %q", status.Error, errMsg)
		}
		return true, ""
	})
}

func (f *pfrFixture) requirePortForwardStarted(name string, localPort int32, containerPort int32) {
	f.t.Helper()
	f.requirePortForwardStatus(name, localPort, containerPort, func(status ForwardStatus) (bool, string) {
		if status.StartedAt.IsZero() || status.Error != "" {
			return false, fmt.Sprintf("status has startedAt=%s / error=%q", status.StartedAt.String(), status.Error)
		}
		return true, ""
	})
}

func (f *pfrFixture) requirePortForwardDeleted(name string) {
	f.t.Helper()
	f.requireState(name, func(pf *PortForward) bool {
		return pf == nil
	}, "port forward deleted")
}

// GetAndUpdate pulls the existing version of the PortForward and issues an
// update (using the ResourceVersion of the existing Port Forward to avoid an
// "object was modified" error)
func (f *pfrFixture) GetAndUpdate(pf *PortForward) ctrl.Result {
	f.t.Helper()
	var existing PortForward
	f.MustGet(f.KeyForObject(pf), &existing)
	pf.SetResourceVersion(existing.GetResourceVersion())
	require.NoError(f.t, f.Client.Update(f.Context(), pf))
	return f.MustReconcile(f.KeyForObject(pf))
}

func (f *pfrFixture) Create(pf *v1alpha1.PortForward) ctrl.Result {
	f.t.Helper()
	f.ensureCluster(pf)
	return f.ControllerFixture.Create(pf)
}

func (f *pfrFixture) assertContextCancelled(t *testing.T, ctx context.Context) {
	if assert.Error(t, ctx.Err(), "expect cancelled context to have a non-nil error") {
		assert.Equal(t, context.Canceled, ctx.Err(), "expect context to be cancelled")
	}
}

func (f *pfrFixture) assertContextNotCancelled(t *testing.T, ctx context.Context) {
	assert.NoError(t, ctx.Err(), "expect non-cancelled context to have no error")
}

func (f *pfrFixture) makePF(name string, mName model.ManifestName, podName k8s.PodID, ns string, forwards []Forward) *PortForward {
	return &PortForward{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: mName.String(),
				v1alpha1.AnnotationSpanID:   string(k8sconv.SpanIDForPod(mName, podName)),
			},
		},
		Spec: PortForwardSpec{
			PodName:   podName.String(),
			Namespace: ns,
			Forwards:  forwards,
		},
	}
}

func (f *pfrFixture) makeSimplePF(name string, localPort, containerPort int32) *PortForward {
	fwd := Forward{
		LocalPort:     localPort,
		ContainerPort: containerPort,
	}
	return f.makeSimplePFMultipleForwards(name, []Forward{fwd})
}

func (f *pfrFixture) makeSimplePFMultipleForwards(name string, forwards []Forward) *PortForward {
	return f.makePF(name, model.ManifestName(fmt.Sprintf("manifest-%s", name)), k8s.PodID(fmt.Sprintf("pod-%s", name)), "", forwards)
}

func (f *pfrFixture) makeForward(localPort, containerPort int32, host string) Forward {
	return Forward{
		LocalPort:     localPort,
		ContainerPort: containerPort,
		Host:          host,
	}
}

func (f *pfrFixture) ensureCluster(pf *v1alpha1.PortForward) {
	f.t.Helper()
	pf = pf.DeepCopy()
	pf.Default()
	f.clients.EnsureK8sCluster(f.Context(), clusterNN(pf))
}
