package cmdimage

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/core/cmd"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestIndexCluster(t *testing.T) {
	f := newFixture(t)

	f.Create(&v1alpha1.CmdImage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-image",
		},
		Spec: v1alpha1.CmdImageSpec{
			Cluster: "cluster",
		},
	})

	ctx := context.Background()
	reqs := f.r.indexer.Enqueue(ctx, &v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}})
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: "my-image"}},
	}, reqs, "Index result for known cluster")

	reqs = f.r.indexer.Enqueue(ctx, &v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "other"}})
	require.Empty(t, reqs, "Index result for unknown cluster")
}

type fixture struct {
	*fake.ControllerFixture
	r *Reconciler
}

func newFixture(t testing.TB) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	clock := clockwork.NewFakeClock()

	fe := cmd.NewProcessExecer(localexec.EmptyEnv())
	fpm := cmd.NewFakeProberManager()
	cmds := cmd.NewController(cfb.Context(), fe, fpm, cfb.Client, cfb.Store, clock, v1alpha1.NewScheme())

	dockerCli := docker.NewFakeClient()
	ib := build.NewImageBuilder(
		build.NewDockerBuilder(dockerCli, nil),
		build.NewCustomBuilder(dockerCli, clock, cmds),
		build.NewKINDLoader())

	r := NewReconciler(cfb.Client, cfb.Store, cfb.Scheme(), docker.NewFakeClient(), ib)
	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
	}
}
