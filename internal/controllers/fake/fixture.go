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
	controller controller
	Scheme     *runtime.Scheme
	Client     ctrlclient.Client
}

func NewFixture(t testing.TB, c controller) *ControllerFixture {
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
		Scheme:     cli.Scheme(),
		Client:     cli,
		controller: c,
	}
}

func (f *ControllerFixture) RootContext() context.Context {
	return f.ctx
}

func (f *ControllerFixture) TimeoutContext() context.Context {
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	f.t.Cleanup(cancel)
	return ctx
}

func (f *ControllerFixture) MustReconcile(name string) ctrl.Result {
	f.t.Helper()
	key := types.NamespacedName{Name: name}
	res, err := f.controller.Reconcile(f.TimeoutContext(), ctrl.Request{NamespacedName: key})
	require.NoError(f.t, err)
	return res
}

func (f *ControllerFixture) Get(name string, out object) bool {
	f.t.Helper()
	err := f.Client.Get(f.ctx, types.NamespacedName{Name: name}, out)
	if apierrors.IsNotFound(err) {
		return false
	}
	require.NoError(f.t, err)
	return true
}

func (f *ControllerFixture) MustGet(name string, out object) {
	f.t.Helper()
	found := f.Get(name, out)
	if !found {
		// don't try to read from object Kind, it's probably not properly populated
		f.t.Fatalf("%T object %q does not exist", out, name)
	}
}

func (f *ControllerFixture) Create(o object) ctrl.Result {
	f.t.Helper()
	require.NoError(f.t, f.Client.Create(f.ctx, o))
	return f.MustReconcile(o.GetName())
}

func (f *ControllerFixture) Update(o object) ctrl.Result {
	f.t.Helper()
	// this is a safe cast since we know the original object type met the interface
	old := o.New().(object)
	key := types.NamespacedName{Name: o.GetObjectMeta().GetName()}
	require.NoError(f.t, f.Client.Get(f.ctx, key, old))
	o.GetObjectMeta().SetResourceVersion(old.GetResourceVersion())
	require.NoError(f.t, f.Client.Update(f.ctx, o))
	return f.MustReconcile(o.GetName())
}

func (f *ControllerFixture) Delete(o object) ctrl.Result {
	f.t.Helper()
	require.NoError(f.t, f.Client.Delete(f.ctx, o))
	return f.MustReconcile(o.GetName())
}
