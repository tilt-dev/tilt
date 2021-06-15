package portforward

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/tilt-dev/tilt/internal/controllers/fake"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/pkg/logger"
)

const (
	pfFooName = "pf_foo"
	pfBarName = "pf_bar"
)

func TestCreatePortForward(t *testing.T) {
	f := newPFRFixture(t)
	defer f.TearDown()

	require.Equal(t, 0, len(f.r.activeForwards))

	pf := f.makeSimplePF(pfFooName, 8000, 8080)
	f.Create(pf)
	f.requirePortForwardWithVersion(pfFooName, 1, "initial PortForward appears")

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", f.kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, f.kCli.LastForwardPortRemotePort())
}

func TestDeletePortForward(t *testing.T) {
	f := newPFRFixture(t)
	defer f.TearDown()

	require.Equal(t, 0, len(f.r.activeForwards))

	pf := f.makeSimplePF(pfFooName, 8000, 8080)
	f.Create(pf)
	f.requirePortForwardWithVersion(pfFooName, 1, "initial PortForward appears")

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", f.kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, f.kCli.LastForwardPortRemotePort())
	origForwardCtx := f.kCli.LastForwardContext()

	f.Delete(pf)
	f.requirePortForwardDeleted(pfFooName)

	require.Equal(t, 0, len(f.r.activeForwards))
	f.assertContextCancelled(t, origForwardCtx)
}

func TestModifyPortForward(t *testing.T) {
	f := newPFRFixture(t)
	defer f.TearDown()

	require.Equal(t, 0, len(f.r.activeForwards))

	pf := f.makeSimplePF(pfFooName, 8000, 8080)
	f.Create(pf)
	f.requirePortForwardWithVersion(pfFooName, 1, "initial PortForward appears")

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", f.kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, f.kCli.LastForwardPortRemotePort())
	origForwardCtx := f.kCli.LastForwardContext()

	pf = f.makeSimplePF(pfFooName, 8000, 9090)
	f.GetAndUpdate(pf)
	f.requirePortForwardWithVersion(pfFooName, 2, "updated PortForward appears")

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", f.kCli.LastForwardPortPodID().String())
	assert.Equal(t, 9090, f.kCli.LastForwardPortRemotePort())

	f.assertContextCancelled(t, origForwardCtx)
}

func TestModifyPortForwardManifestName(t *testing.T) {
	// A change to only the manifestName should be enough to tear down and recreate
	// a PortForward (we need to do this so the logs will be routed correctly)
	f := newPFRFixture(t)
	defer f.TearDown()

	require.Equal(t, 0, len(f.r.activeForwards))

	fwds := []v1alpha1.Forward{f.makeForward(8000, 8080, "")}

	pf := f.makePF(pfFooName, "manifestA", "pod-pf_foo", "", fwds)
	f.Create(pf)
	f.requirePortForwardWithVersion(pfFooName, 1, "initial PortForward appears")

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", f.kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, f.kCli.LastForwardPortRemotePort())
	origForwardCtx := f.kCli.LastForwardContext()

	pf = f.makePF(pfFooName, "manifestB", "pod-pf_foo", "", fwds)
	f.GetAndUpdate(pf)
	f.requirePortForwardWithVersion(pfFooName, 2, "updated PortForward appears")

	require.Equal(t, 1, len(f.r.activeForwards))
	assert.Equal(t, "pod-pf_foo", f.kCli.LastForwardPortPodID().String())
	assert.Equal(t, 8080, f.kCli.LastForwardPortRemotePort())

	f.assertContextCancelled(t, origForwardCtx)
}

func TestMultipleForwardsForOnePod(t *testing.T) {
	f := newPFRFixture(t)
	defer f.TearDown()

	require.Equal(t, 0, len(f.r.activeForwards))

	forwards := []v1alpha1.Forward{
		f.makeForward(8000, 8080, "hostA"),
		f.makeForward(8001, 8081, "hostB"),
	}

	pf := f.makeSimplePFMultipleForwards(pfFooName, forwards)
	f.Create(pf)
	f.requirePortForwardWithVersion(pfFooName, 1, "initial PortForward appears")

	require.Equal(t, 1, len(f.r.activeForwards))
	require.Equal(t, 2, f.kCli.CreatePortForwardCallCount())

	var seen8080, seen8081 bool
	var contexts []context.Context
	for _, call := range f.kCli.PortForwardCalls() {
		assert.Equal(t, "pod-pf_foo", call.PodID.String())
		if call.RemotePort == 8080 {
			seen8080 = true
			contexts = append(contexts, call.Context)
			assert.Equal(t, "hostA", call.Host, "unexpected host for port forward to 8080")
		} else if call.RemotePort == 8081 {
			seen8081 = true
			contexts = append(contexts, call.Context)
			assert.Equal(t, "hostB", call.Host, "unexpected host for port forward to 8081")
		} else {
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
	defer f.TearDown()

	require.Equal(t, 0, len(f.r.activeForwards))

	fwdsFoo := []v1alpha1.Forward{f.makeForward(8000, 8080, "host-foo")}
	fwdsBar := []v1alpha1.Forward{f.makeForward(8001, 8081, "host-bar")}
	pfFoo := f.makePF(pfFooName, "foo", "pod-pf_foo", "ns-foo", fwdsFoo)
	pfBar := f.makePF(pfBarName, "bar", "pod-pf_bar", "ns-bar", fwdsBar)
	f.Create(pfFoo)
	f.Create(pfBar)
	f.requirePortForwardWithVersion(pfFooName, 1, "initial API object pfFoo appears")
	f.requirePortForwardWithVersion(pfBarName, 1, "initial API object pfBar appears")

	require.Equal(t, 2, len(f.r.activeForwards))
	require.Equal(t, 2, f.kCli.CreatePortForwardCallCount())

	// PortForwards are executed async so we can't guarantee the order;
	// just make sure each expected call appears exactly once
	var seenFoo, seenBar bool
	var ctxFoo, ctxBar context.Context
	for _, call := range f.kCli.PortForwardCalls() {
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

type pfrFixture struct {
	*fake.ControllerFixture
	t      *testing.T
	ctx    context.Context
	cancel func()
	kCli   *k8s.FakeK8sClient
	st     *store.TestingStore
	r      *Reconciler
	out    *bufsync.ThreadSafeBuffer
}

func newPFRFixture(t *testing.T) *pfrFixture {
	st := store.NewTestingStore()
	kCli := k8s.NewFakeK8sClient(t)
	r := NewReconciler(st, kCli)
	cf := fake.NewControllerFixture(t, r)
	out := bufsync.NewThreadSafeBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.WithLogger(ctx, logger.NewTestLogger(out))
	return &pfrFixture{
		ControllerFixture: cf,
		t:                 t,
		ctx:               ctx,
		cancel:            cancel,
		st:                st,
		kCli:              kCli,
		r:                 r,
		out:               out,
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
	}, time.Second, 20*time.Millisecond, msg, args...)
}

func (f *pfrFixture) requirePortForwardWithVersion(name string, version int, msg string, args ...interface{}) {
	f.t.Helper()

	newMsg := fmt.Sprintf("[want PortForward.ResourceVersion == %d] %s", version, msg)
	f.requireState(name, func(pf *PortForward) bool {
		return pf.ResourceVersion == strconv.Itoa(version)
	}, newMsg, args...)
}

func (f *pfrFixture) requirePortForwardDeleted(name string) {
	f.t.Helper()
	f.requireState(name, func(pf *PortForward) bool {
		return pf == nil
	}, "port forward deleted")
}

func (f *pfrFixture) TearDown() {
	f.kCli.TearDown()
	f.cancel()
}

// GetAndUpdate pulls the existing version of the PortForward and issues an
// update (using the ResourceVersion of the existing Port Forward to avoid an
// "object was modified" error)
func (f *pfrFixture) GetAndUpdate(pf *PortForward) ctrl.Result {
	f.t.Helper()
	var existing PortForward
	f.MustGet(f.KeyForObject(pf), &existing)
	pf.SetResourceVersion(existing.GetResourceVersion())
	require.NoError(f.t, f.Client.Update(f.ctx, pf))
	return f.MustReconcile(f.KeyForObject(pf))
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
