package extension

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/wmclient/pkg/analytics"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestDefault(t *testing.T) {
	f := newFixture(t)
	repo := f.setupRepo()

	ext := v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-repo:my-ext",
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: "my-repo",
			RepoPath: "my-ext",
			Args:     []string{"--namespaces=foo"},
		},
	}
	f.Create(&ext)

	f.MustGet(types.NamespacedName{Name: "my-repo:my-ext"}, &ext)

	p := f.JoinPath("my-repo", "my-ext", "Tiltfile")
	require.Equal(t, p, ext.Status.Path)

	var tf v1alpha1.Tiltfile
	f.MustGet(types.NamespacedName{Name: "my-repo:my-ext"}, &tf)
	require.Equal(t, tf.Spec, v1alpha1.TiltfileSpec{
		Path:      p,
		Labels:    map[string]string{"extension.my-repo_my-ext": "extension.my-repo_my-ext"},
		RestartOn: &v1alpha1.RestartOnSpec{FileWatches: []string{"configs:my-repo:my-ext"}},
		Args:      []string{"--namespaces=foo"},
	})

	assert.Equal(t, f.ma.Counts, []analytics.CountEvent{
		{
			Name: "api.extension.load",
			Tags: map[string]string{
				"ext_path":      "my-ext",
				"repo_type":     "file",
				"repo_url_hash": tiltanalytics.HashSHA1(repo.Spec.URL),
			},
			N: 1,
		},
	})
}

// Assert that the repo extension path, if specified, is used for extension location
func TestRepoSubpath(t *testing.T) {
	f := newFixture(t)
	repo := f.setupRepoSubpath("subpath")

	ext := v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-repo:my-ext",
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: "my-repo",
			RepoPath: "my-ext",
			Args:     []string{"--namespaces=foo"},
		},
	}
	f.Create(&ext)

	f.MustGet(types.NamespacedName{Name: "my-repo:my-ext"}, &ext)

	p := f.JoinPath("my-repo", "subpath", "my-ext", "Tiltfile")
	require.Equal(t, p, ext.Status.Path)

	var tf v1alpha1.Tiltfile
	f.MustGet(types.NamespacedName{Name: "my-repo:my-ext"}, &tf)
	require.Equal(t, tf.Spec, v1alpha1.TiltfileSpec{
		Path:      p,
		Labels:    map[string]string{"extension.my-repo_my-ext": "extension.my-repo_my-ext"},
		RestartOn: &v1alpha1.RestartOnSpec{FileWatches: []string{"configs:my-repo:my-ext"}},
		Args:      []string{"--namespaces=foo"},
	})

	assert.Equal(t, f.ma.Counts, []analytics.CountEvent{
		{
			Name: "api.extension.load",
			Tags: map[string]string{
				"ext_path":      "my-ext",
				"repo_type":     "file",
				"repo_url_hash": tiltanalytics.HashSHA1(repo.Spec.URL),
			},
			N: 1,
		},
	})
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

// Verify that args are propagated to the tiltfile when changed.
func TestChangeArgs(t *testing.T) {
	f := newFixture(t)
	f.setupRepo()

	ext := v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ext",
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: "my-repo",
			RepoPath: "my-ext",
			Args:     []string{"--namespace=bar"},
		},
	}
	f.Create(&ext)

	f.MustGet(types.NamespacedName{Name: "ext"}, &ext)

	ext.Spec.Args = []string{"--namespace=foo"}
	f.Update(&ext)

	var tf v1alpha1.Tiltfile
	f.MustGet(types.NamespacedName{Name: "ext"}, &tf)
	require.Equal(t, []string{"--namespace=foo"}, tf.Spec.Args)
}

// Verify that no errors get printed if the extension
// appears in the apiserver before the repo appears.
func TestExtensionBeforeRepo(t *testing.T) {
	f := newFixture(t)

	nn := types.NamespacedName{Name: "ext"}
	ext := v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ext",
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: "my-repo",
			RepoPath: "my-ext",
		},
	}
	f.Create(&ext)

	f.MustGet(nn, &ext)
	assert.Equal(t, "extension repo not found: my-repo", ext.Status.Error)
	assert.Equal(t, "", f.Stdout())

	f.setupRepo()
	f.MustReconcile(nn)
	f.MustGet(nn, &ext)
	assert.Equal(t, "", ext.Status.Error)
	assert.Equal(t, f.JoinPath("my-repo", "my-ext", "Tiltfile"), ext.Status.Path)
	assert.Equal(t, "", f.Stdout())
}

type fixture struct {
	*fake.ControllerFixture
	*tempdir.TempDirFixture
	r  *Reconciler
	ma *analytics.MemoryAnalytics
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	tf := tempdir.NewTempDirFixture(t)

	o := tiltanalytics.NewFakeOpter(analytics.OptIn)
	ma, ta := tiltanalytics.NewMemoryTiltAnalyticsForTest(o)

	r := NewReconciler(cfb.Client, v1alpha1.NewScheme(), ta)
	return &fixture{
		ControllerFixture: cfb.Build(r),
		TempDirFixture:    tf,
		r:                 r,
		ma:                ma,
	}
}

func (f *fixture) setupRepo() *v1alpha1.ExtensionRepo {
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
	return &repo
}

func (f *fixture) setupRepoSubpath(subpath string) *v1alpha1.ExtensionRepo {
	p := f.JoinPath("my-repo", subpath, "my-ext", "Tiltfile")
	f.WriteFile(p, "print('hello-world')")

	repo := v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-repo",
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL:        fmt.Sprintf("file://%s", f.JoinPath("my-repo")),
			GitSubpath: subpath,
		},
		Status: v1alpha1.ExtensionRepoStatus{
			Path: f.JoinPath("my-repo"),
		},
	}
	f.Create(&repo)
	return &repo
}
