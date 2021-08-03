package extensionrepo

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/wmclient/pkg/dirs"
	"github.com/tilt-dev/wmclient/pkg/os/temp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestInvalidRepo(t *testing.T) {
	f := newFixture(t)
	key := types.NamespacedName{Name: "default"}
	repo := v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: key.Name,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: "x",
		},
	}
	f.Create(&repo)
	f.MustGet(key, &repo)
	require.Equal(t, "URL must start with 'https://': x", repo.Status.Error)
}

func TestUnknown(t *testing.T) {
	f := newFixture(t)
	key := types.NamespacedName{Name: "default"}
	repo := v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: key.Name,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: "https://github.com/tilt-dev/unknown-repo",
		},
	}
	f.Create(&repo)
	f.MustGet(key, &repo)
	require.Contains(t, repo.Status.Error, "Downloading ExtensionRepo default. Waiting 5s before retrying.")
}

func TestDefault(t *testing.T) {
	f := newFixture(t)
	key := types.NamespacedName{Name: "default"}
	repo := v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: key.Name,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: "https://github.com/tilt-dev/tilt-extensions",
		},
	}
	f.Create(&repo)
	f.MustGet(key, &repo)
	require.Equal(t, repo.Status.Error, "")
	require.True(t, strings.HasSuffix(repo.Status.Path, "tilt-extensions"))

	info, err := os.Stat(repo.Status.Path)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	f.Delete(&repo)

	_, err = os.Stat(repo.Status.Path)
	require.True(t, os.IsNotExist(err))
}

type fixture struct {
	*fake.ControllerFixture
	r *Reconciler
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	tmpDir, err := temp.NewDir(tempdir.SanitizeFileName(t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir.Path()) })

	dir := dirs.NewTiltDevDirAt(tmpDir.Path())
	r, err := NewReconciler(cfb.Client, dir)
	require.NoError(t, err)

	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
	}
}
