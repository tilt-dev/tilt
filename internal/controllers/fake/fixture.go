package fake

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// controller just exists to prevent an import cycle for controllers.
// It's not exported and should match the minimal set of methods needed from controllers.Controller.
type controller interface {
	reconcile.Reconciler
	SetClient(client ctrlclient.Client)
}

// object just bridges together a couple of different representations of runtime.Object.
// Scaffolded/code-generated types should meet this by default.
type object interface {
	ctrlclient.Object
	resource.Object
}

type ControllerFixture struct {
	t          testing.TB
	ctx        context.Context
	cancel     context.CancelFunc
	controller controller
	Scheme     *runtime.Scheme
	Client     ctrlclient.Client
}

func NewControllerFixture(t testing.TB, c controller) *ControllerFixture {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx = logger.WithLogger(ctx, l)

	cli := NewTiltClient()
	c.SetClient(cli)

	return &ControllerFixture{
		t:          t,
		ctx:        ctx,
		cancel:     cancel,
		Scheme:     cli.Scheme(),
		Client:     cli,
		controller: c,
	}
}

// Cancel cancels the internal context used for the controller and client requests.
//
// Normally, it's not necessary to call this - the fixture will automatically cancel the context as part of test
// cleanup to avoid leaking resources. However, if you want to explicitly test how a controller reacts to context
// cancellation, this method can be used.
func (f ControllerFixture) Cancel() {
	f.cancel()
}

func (f *ControllerFixture) RootContext() context.Context {
	return f.ctx
}

func (f *ControllerFixture) TimeoutContext() context.Context {
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	f.t.Cleanup(cancel)
	return ctx
}

func (f *ControllerFixture) KeyForObject(o object) types.NamespacedName {
	return types.NamespacedName{Namespace: o.GetNamespace(), Name: o.GetName()}
}

func (f *ControllerFixture) MustReconcile(key types.NamespacedName) ctrl.Result {
	f.t.Helper()
	res, err := f.controller.Reconcile(f.TimeoutContext(), ctrl.Request{NamespacedName: key})
	require.NoError(f.t, err)
	return res
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
