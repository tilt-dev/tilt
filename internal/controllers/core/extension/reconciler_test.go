package extension

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestDefault(t *testing.T) {
	f := newFixture(t)
	f.setupRepo()

	ext := v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-repo:my-ext",
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: "my-repo",
			RepoPath: "my-ext",
		},
	}
	f.Create(&ext)

	f.MustGet(types.NamespacedName{Name: "my-repo:my-ext"}, &ext)

	p := f.JoinPath("my-repo", "my-ext", "Tiltfile")
	require.Equal(t, p, ext.Status.Path)

	var tf v1alpha1.Tiltfile
	f.MustGet(types.NamespacedName{Name: "my-repo:my-ext"}, &tf)
	require.Equal(t, p, tf.Spec.Path)
}

func TestCleanupTiltfile(t *testing.T) {
	f := newFixture(t)
	f.setupRepo()

	ext := v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-repo:my-ext",
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: "my-repo",
			RepoPath: "my-ext",
		},
	}
	f.Create(&ext)

	f.MustGet(types.NamespacedName{Name: "my-repo:my-ext"}, &ext)

	p := f.JoinPath("my-repo", "my-ext", "Tiltfile")

	var tf v1alpha1.Tiltfile
	f.MustGet(types.NamespacedName{Name: "my-repo:my-ext"}, &tf)
	require.Equal(t, p, tf.Spec.Path)

	f.Delete(&ext)
	assert.False(t, f.Get(types.NamespacedName{Name: "my-repo:my-ext"}, &tf))
}

func TestMissing(t *testing.T) {
	f := newFixture(t)
	f.setupRepo()

	ext := v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-repo:my-ext",
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: "my-repo",
			RepoPath: "my-ext2",
		},
	}
	f.Create(&ext)

	f.MustGet(types.NamespacedName{Name: "my-repo:my-ext"}, &ext)
	p := f.JoinPath("my-repo", "my-ext2", "Tiltfile")
	require.Equal(t, ext.Status.Error,
		fmt.Sprintf("no extension tiltfile found at %s", p), ext.Status.Error)
}

type fixture struct {
	*fake.ControllerFixture
	*tempdir.TempDirFixture
	r *Reconciler
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	tf := tempdir.NewTempDirFixture(t)
	t.Cleanup(tf.TearDown)

	r := NewReconciler(cfb.Client, v1alpha1.NewScheme())
	return &fixture{
		ControllerFixture: cfb.Build(r),
		TempDirFixture:    tf,
		r:                 r,
	}
}

func (f *fixture) setupRepo() {
	p := f.JoinPath("my-repo", "my-ext", "Tiltfile")
	f.WriteFile(p, "print('hello-world')")

	repo := v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-repo",
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: fmt.Sprintf("file://%s", f.JoinPath("my-repo")),
		},
		Status: v1alpha1.ExtensionRepoStatus{
			Path: f.JoinPath("my-repo"),
		},
	}
	f.Create(&repo)
}
