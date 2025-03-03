package extensionrepo

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/xdg"
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

	path := filepath.Join(f.base.Dir, "my-repo")
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
	f.assertSteadyState(&repo)
}

func TestRepoSyncExisting(t *testing.T) {
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

	f.dlr.Download("github.com/tilt-dev/tilt-extensions")
	f.dlr.RefSync("github.com/tilt-dev/tilt-extensions", "other-ref")

	f.Create(&repo)
	f.MustGet(key, &repo)
	require.Equal(t, repo.Status.Error, "")
	assert.Equal(t, 1, f.dlr.downloadCount)
	assert.Equal(t, "other-ref", f.dlr.lastRefSync)
	f.assertSteadyState(&repo)
}

func TestRepoAlwaysSyncHead(t *testing.T) {
	f := newFixture(t)

	key := types.NamespacedName{Name: "default"}
	repo := v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: key.Name,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: "https://github.com/tilt-dev/tilt-extensions",
			Ref: "HEAD",
		},
	}

	f.dlr.Download("github.com/tilt-dev/tilt-extensions")
	f.dlr.RefSync("github.com/tilt-dev/tilt-extensions", "HEAD")

	f.Create(&repo)
	f.MustGet(key, &repo)
	require.Equal(t, repo.Status.Error, "")
	assert.Equal(t, 2, f.dlr.downloadCount)
	assert.Equal(t, "HEAD", f.dlr.lastRefSync)
	f.assertSteadyState(&repo)
}

func TestStale(t *testing.T) {
	f := newFixture(t)

	f.dlr.Download("github.com/tilt-dev/stale-repo")
	f.dlr.downloadError = fmt.Errorf("fake error")

	key := types.NamespacedName{Name: "default"}
	repo := v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: key.Name,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: "https://github.com/tilt-dev/stale-repo",
		},
	}
	f.Create(&repo)
	f.MustGet(key, &repo)
	require.Contains(t, repo.Status.Error, "")
	require.Contains(t, repo.Status.StaleReason, "fake error")
}

type fixture struct {
	*fake.ControllerFixture
	r    *Reconciler
	dlr  *fakeDownloader
	base *xdg.FakeBase
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	tmpDir := t.TempDir()
	fs := afero.NewOsFs()
	base := xdg.NewFakeBase(tmpDir, fs)
	r, err := NewReconciler(cfb.Client, cfb.Store, base)
	require.NoError(t, err)

	dlr := &fakeDownloader{base: base, headRef: "fake-head"}
	r.dlr = dlr

	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
		dlr:               dlr,
		base:              base,
	}
}

func (f *fixture) assertSteadyState(er *v1alpha1.ExtensionRepo) {
	f.T().Helper()
	f.MustReconcile(types.NamespacedName{Name: er.Name})
	var er2 v1alpha1.ExtensionRepo
	f.MustGet(types.NamespacedName{Name: er.Name}, &er2)
	assert.Equal(f.T(), er.ResourceVersion, er2.ResourceVersion)
}

type fakeDownloader struct {
	base xdg.Base

	downloadError error
	downloadCount int
	lastRefSync   string
	headRef       string
}

func (d *fakeDownloader) DestinationPath(pkg string) string {
	result, _ := d.base.DataFile(pkg)
	return result
}

func (d *fakeDownloader) Download(pkg string) (string, error) {

	d.downloadCount += 1
	if d.downloadError != nil {
		return "", fmt.Errorf("download error %d: %v", d.downloadCount, d.downloadError)
	}

	path, err := d.base.DataFile(filepath.Join(pkg, "Tiltfile"))
	if err != nil {
		return "", err
	}

	_, err = os.Stat(path)
	exists := err == nil
	if exists && d.lastRefSync != "" && d.lastRefSync != "HEAD" {
		// If the current disk state is checked out to a ref, then
		// we expect Download() to fail.
		// https://github.com/tilt-dev/tilt/issues/5508
		return "", fmt.Errorf("You are not currently on a branch.")
	}

	err = os.WriteFile(path,
		[]byte(fmt.Sprintf("Download count %d", d.downloadCount)), fs.FileMode(0777))
	return "", err
}

func (d *fakeDownloader) HeadRef(pkg string) (string, error) {
	return d.headRef, nil
}

func (d *fakeDownloader) RefSync(pkg string, ref string) error {
	d.lastRefSync = ref
	return nil
}
