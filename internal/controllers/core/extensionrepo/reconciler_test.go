package extensionrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, "invalid: URL must start with 'https://': x", repo.Status.Error)
	assert.Equal(t, "extensionrepo default: invalid: URL must start with 'https://': x\n", f.Stdout())
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
	require.Contains(t, repo.Status.Error, "download error: waiting 5s before retrying")
}

func TestDefaultWeb(t *testing.T) {
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

func TestDefaultFile(t *testing.T) {
	f := newFixture(t)

	path := filepath.Join(f.dir.Path(), "my-repo")
	_ = os.MkdirAll(path, os.FileMode(0755))

	key := types.NamespacedName{Name: "default"}
	repo := v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: key.Name,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: fmt.Sprintf("file://%s", path),
		},
	}
	f.Create(&repo)
	f.MustGet(key, &repo)
	require.Equal(t, repo.Status.Error, "")
	require.Equal(t, repo.Status.Path, path)
}

type fixture struct {
	*fake.ControllerFixture
	r   *Reconciler
	dir *temp.TempDir
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
		dir:               tmpDir,
	}
}
