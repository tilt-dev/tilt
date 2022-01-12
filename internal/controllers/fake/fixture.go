package fake

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/wmclient/pkg/analytics"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
)

// controller just exists to prevent an import cycle for controllers.
// It's not exported and should match the minimal set of methods needed from controllers.Controller.
type controller interface {
	reconcile.Reconciler
	CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error)
}

// object just bridges together a couple of different representations of runtime.Object.
// Scaffolded/code-generated types should meet this by default.
type object interface {
	ctrlclient.Object
	resource.Object
}

type ControllerFixture struct {
	t          testing.TB
	out        *bufsync.ThreadSafeBuffer
	ctx        context.Context
	cancel     context.CancelFunc
	controller controller
	Store      store.RStore
	Scheme     *runtime.Scheme
	Client     ctrlclient.Client
}

type ControllerFixtureBuilder struct {
	t      testing.TB
	ctx    context.Context
	cancel context.CancelFunc
	out    *bufsync.ThreadSafeBuffer
	ma     *analytics.MemoryAnalytics
	Client ctrlclient.Client
	Store  store.RStore
}

func NewControllerFixtureBuilder(t testing.TB) *ControllerFixtureBuilder {
	out := bufsync.NewThreadSafeBuffer()

	ctx, ma, _ := testutils.ForkedCtxAndAnalyticsForTest(out)

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	return &ControllerFixtureBuilder{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		out:    out,
		ma:     ma,
		Client: NewFakeTiltClient(),
		Store:  NewTestingStore(out),
	}
}

func (b ControllerFixtureBuilder) Scheme() *runtime.Scheme {
	return b.Client.Scheme()
}

func (b ControllerFixtureBuilder) Analytics() *analytics.MemoryAnalytics {
	return b.ma
}

func (b ControllerFixtureBuilder) Build(c controller) *ControllerFixture {
	b.t.Helper()

	// apiserver controller initialization is awkward and some parts are done via the builder,
	// so we call it here even though we won't actually use the builder result
	// currently, this relies on the fact that no controllers actually use the
	// controllerruntime.Manager argument for anything besides passing it along - if that changes,
	// we'll need to provide a mock of it that implements the requisite functionality
	_, err := c.CreateBuilder(nil)
	require.NoError(b.t, err, "Error in controller CreateBuilder()")

	return &ControllerFixture{
		t:          b.t,
		out:        b.out,
		ctx:        b.ctx,
		cancel:     b.cancel,
		Scheme:     b.Client.Scheme(),
		Client:     b.Client,
		Store:      b.Store,
		controller: c,
	}
}

func (b ControllerFixtureBuilder) OutWriter() io.Writer {
	return b.out
}

func (b ControllerFixtureBuilder) Context() context.Context {
	return b.ctx
}

func (b ControllerFixture) Stdout() string {
	return b.out.String()
}

func (f ControllerFixture) T() testing.TB {
	return f.t
}

// Cancel cancels the internal context used for the controller and client requests.
//
// Normally, it's not necessary to call this - the fixture will automatically cancel the context as part of test
// cleanup to avoid leaking resources. However, if you want to explicitly test how a controller reacts to context
// cancellation, this method can be used.
func (f ControllerFixture) Cancel() {
	f.cancel()
}

func (f *ControllerFixture) Context() context.Context {
	return f.ctx
}

func (f *ControllerFixture) KeyForObject(o object) types.NamespacedName {
	return types.NamespacedName{Namespace: o.GetNamespace(), Name: o.GetName()}
}

func (f *ControllerFixture) MustReconcile(key types.NamespacedName) ctrl.Result {
	f.t.Helper()
	result, err := f.Reconcile(key)
	require.NoError(f.t, err)
	return result
}

func (f *ControllerFixture) Reconcile(key types.NamespacedName) (ctrl.Result, error) {
	f.t.Helper()
	return f.controller.Reconcile(f.ctx, ctrl.Request{NamespacedName: key})
}

func (f *ControllerFixture) ReconcileWithErrors(key types.NamespacedName, expectedErrorSubstrings ...string) {
	f.t.Helper()
	_, err := f.Reconcile(key)
	require.Error(f.t, err)
	for _, s := range expectedErrorSubstrings {
		require.Contains(f.t, err.Error(), s)
	}
}

func (f *ControllerFixture) Get(key types.NamespacedName, out object) bool {
	f.t.Helper()
	err := f.Client.Get(f.ctx, key, out)
	if apierrors.IsNotFound(err) {
		return false
	}
	require.NoError(f.t, err)
	return true
}

func (f *ControllerFixture) MustGet(key types.NamespacedName, out object) {
	f.t.Helper()
	found := f.Get(key, out)
	if !found {
		// don't try to read from object Kind, it's probably not properly populated
		f.t.Fatalf("%T object %q does not exist", out, key.String())
	}
}

func (f *ControllerFixture) List(out ctrlclient.ObjectList) {
	f.t.Helper()
	err := f.Client.List(f.ctx, out)
	require.NoError(f.t, err)
}

func (f *ControllerFixture) Create(o object) ctrl.Result {
	f.t.Helper()
	require.NoError(f.t, f.Client.Create(f.ctx, o))
	return f.MustReconcile(f.KeyForObject(o))
}

// Update updates the object metadata and spec.
func (f *ControllerFixture) Update(o object) ctrl.Result {
	f.t.Helper()
	require.NoError(f.t, f.Client.Update(f.ctx, o))
	return f.MustReconcile(f.KeyForObject(o))
}

// Create or update.
func (f *ControllerFixture) Upsert(o object) ctrl.Result {
	f.t.Helper()

	err := f.Client.Create(f.ctx, o)
	if apierrors.IsAlreadyExists(err) {
		update := o.DeepCopyObject().(object)

		require.NoError(f.t, f.Client.Get(f.ctx, f.KeyForObject(o), o))
		update.SetResourceVersion(o.GetResourceVersion())
		return f.Update(update)
	}
	require.NoError(f.t, err)
	return f.MustReconcile(f.KeyForObject(o))
}

func (f *ControllerFixture) UpdateStatus(o object) ctrl.Result {
	f.t.Helper()
	require.NoError(f.t, f.Client.Status().Update(f.ctx, o))
	return f.MustReconcile(f.KeyForObject(o))
}

func (f *ControllerFixture) Delete(o object) (bool, ctrl.Result) {
	f.t.Helper()
	err := f.Client.Delete(f.ctx, o)
	require.NoError(f.t, ctrlclient.IgnoreNotFound(err))
	if apierrors.IsNotFound(err) {
		// skip reconciliation since no object was deleted
		return false, ctrl.Result{}
	}
	return true, f.MustReconcile(f.KeyForObject(o))
}
