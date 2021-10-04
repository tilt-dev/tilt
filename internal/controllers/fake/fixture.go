package fake

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"

	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// controller just exists to prevent an import cycle for controllers.
// It's not exported and should match the minimal set of methods needed from controllers.Controller.
type controller interface {
	reconcile.Reconciler
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
	Scheme     *runtime.Scheme
	Client     ctrlclient.Client
}

type ControllerFixtureBuilder struct {
	t      testing.TB
	Client ctrlclient.Client
}

func NewControllerFixtureBuilder(t testing.TB) *ControllerFixtureBuilder {
	return &ControllerFixtureBuilder{
		t:      t,
		Client: NewFakeTiltClient(),
	}
}

func (b ControllerFixtureBuilder) Build(c controller) *ControllerFixture {
	return newControllerFixture(b.t, b.Client, c)
}

func newControllerFixture(t testing.TB, cli ctrlclient.Client, c controller) *ControllerFixture {
	t.Helper()

	out := bufsync.NewThreadSafeBuffer()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ctx = logger.WithLogger(ctx, logger.NewTestLogger(io.MultiWriter(out, os.Stdout)))

	return &ControllerFixture{
		t:          t,
		out:        out,
		ctx:        ctx,
		cancel:     cancel,
		Scheme:     cli.Scheme(),
		Client:     cli,
		controller: c,
	}
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

// Update updates the object including Status subresource.
func (f *ControllerFixture) Update(o object) ctrl.Result {
	f.t.Helper()
	require.NoError(f.t, f.Client.Update(f.ctx, o))
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
