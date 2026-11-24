package cmdimage

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/core/cmd"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
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

// Each resource that uses an image gets its own CmdImage, and all of them
// write to the one ImageMap for that image.
func TestImageMapKeepsNewestBuild(t *testing.T) {
	f := newFixture(t)

	iTarget := model.MustNewImageTarget(container.MustParseSelector("my-image"))
	imageMapNN := types.NamespacedName{Name: iTarget.ImageMapName()}
	require.NoError(t, f.Client.Create(f.Context(), &v1alpha1.ImageMap{
		ObjectMeta: metav1.ObjectMeta{Name: imageMapNN.Name},
	}))

	a := types.NamespacedName{Name: "a:my-image"}
	b := types.NamespacedName{Name: "b:my-image"}
	f.Create(&v1alpha1.CmdImage{ObjectMeta: metav1.ObjectMeta{Name: a.Name}})
	f.Create(&v1alpha1.CmdImage{ObjectMeta: metav1.ObjectMeta{Name: b.Name}})

	imageMapImage := func() string {
		var im v1alpha1.ImageMap
		f.MustGet(imageMapNN, &im)
		return im.Status.Image
	}

	firstBuild := metav1.NewMicroTime(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	secondBuild := metav1.NewMicroTime(firstBuild.Add(time.Minute))

	f.r.setImageMapStatus(b, iTarget, v1alpha1.ImageMapStatus{
		Image:          "my-image:first",
		BuildStartTime: &firstBuild,
	})
	f.MustReconcile(b)
	require.Equal(t, "my-image:first", imageMapImage())

	f.r.setImageMapStatus(a, iTarget, v1alpha1.ImageMapStatus{
		Image:          "my-image:second",
		BuildStartTime: &secondBuild,
	})
	f.MustReconcile(a)
	require.Equal(t, "my-image:second", imageMapImage())

	// b has not built again, so its last build is still the first one.
	f.MustReconcile(b)
	require.Equal(t, "my-image:second", imageMapImage())
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
