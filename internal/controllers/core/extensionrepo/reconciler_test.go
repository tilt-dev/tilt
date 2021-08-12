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
	f.dlr.downloadError = fmt.Errorf("X")
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
	require.Equal(t, repo.Status.CheckoutRef, "fake-head")

	info, err := os.Stat(repo.Status.Path)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	assert.Equal(t, 1, f.dlr.downloadCount)
	assert.Equal(t, "", f.dlr.lastRefSync)

	f.MustReconcile(key)
	assert.Equal(t, 1, f.dlr.downloadCount)

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

func TestRepoSync(t *testing.T) {
	f := newFixture(t)

	key := types.NamespacedName{Name: "default"}
	repo := v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: key.Name,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: "https://github.com/tilt-dev/tilt-extensions",
			Ref: "other-ref",
		},
	}
	f.Create(&repo)
	f.MustGet(key, &repo)
	require.Equal(t, repo.Status.Error, "")
	assert.Equal(t, 1, f.dlr.downloadCount)
	assert.Equal(t, "other-ref", f.dlr.lastRefSync)
}

type fixture struct {
	*fake.ControllerFixture
	r   *Reconciler
	dir *temp.TempDir
	dlr *fakeDownloader
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	tmpDir, err := temp.NewDir(tempdir.SanitizeFileName(t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir.Path()) })

	dir := dirs.NewTiltDevDirAt(tmpDir.Path())
	r, err := NewReconciler(cfb.Client, dir)
	require.NoError(t, err)

	dlr := &fakeDownloader{dir: dir, headRef: "fake-head"}
	r.dlr = dlr

	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
		dir:               tmpDir,
		dlr:               dlr,
	}
}

type fakeDownloader struct {
	dir *dirs.TiltDevDir

	downloadError error
	downloadCount int
	lastRefSync   string
	headRef       string
}

func (d *fakeDownloader) DestinationPath(pkg string) string {
	result, _ := d.dir.Abs(pkg)
	return result
}

func (d *fakeDownloader) Download(pkg string) (string, error) {
	d.downloadCount = d.downloadCount + 1
	if d.downloadError != nil {
		return "", fmt.Errorf("download error %d: %v", d.downloadCount, d.downloadError)
	}

	err := d.dir.WriteFile(filepath.Join(pkg, "Tiltfile"),
		fmt.Sprintf("Download count %d", d.downloadCount))
	return "", err
}

func (d *fakeDownloader) HeadRef(pkg string) (string, error) {
	return d.headRef, nil
}

func (d *fakeDownloader) RefSync(pkg string, ref string) error {
	d.lastRefSync = ref
	return nil
}
