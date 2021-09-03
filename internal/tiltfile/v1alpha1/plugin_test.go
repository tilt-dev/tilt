package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestExtension(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
v1alpha1.extension_repo(name='default', url='https://github.com/tilt-dev/tilt-extensions', ref='HEAD')
v1alpha1.extension(name='cancel', repo_name='default', repo_path='cancel')
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	repo := set[(&v1alpha1.ExtensionRepo{}).GetGroupVersionResource()]["default"].(*v1alpha1.ExtensionRepo)
	require.NotNil(t, repo)
	require.Equal(t, "https://github.com/tilt-dev/tilt-extensions", repo.Spec.URL)
	require.Equal(t, "HEAD", repo.Spec.Ref)

	ext := set[(&v1alpha1.Extension{}).GetGroupVersionResource()]["cancel"].(*v1alpha1.Extension)
	require.NotNil(t, ext)
	require.Equal(t, "default", ext.Spec.RepoName)
}

func TestExtensionArgs(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
v1alpha1.extension_repo(name='default', url='https://github.com/tilt-dev/tilt-extensions', ref='HEAD')
v1alpha1.extension(name='cancel', repo_name='default', repo_path='cancel', args=['--namespace=foo'])
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	set := MustState(result)

	ext := set[(&v1alpha1.Extension{}).GetGroupVersionResource()]["cancel"].(*v1alpha1.Extension)
	require.NotNil(t, ext)
	require.Equal(t, []string{"--namespace=foo"}, ext.Spec.Args)
}

func TestExtensionValidation(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
v1alpha1.extension_repo(name='default', url='ftp://github.com/tilt-dev/tilt-extensions')
`)
	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "URLs must start with http(s):// or file://")
}

func newFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewPlugin())
}
