package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lsp.dev/uri"

	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile/tiltextension"
)

func TestReadDocument(t *testing.T) {
	f := newFixture(t)
	f.MkdirAll("tilt-extensions/hello")
	contents := `hello = "Hi"`
	f.WriteFile("tilt-extensions/hello/Tiltfile", contents)

	bytes, err := f.finder.readDocument(uri.URI("ext://hello"))
	require.NoError(t, err)
	assert.Equal(t, contents, string(bytes))

	_, err = f.finder.readDocument(uri.URI("ext://world"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	bytes, err = f.finder.readDocument(uri.File(filepath.Join(f.Path(), "tilt-extensions/hello/Tiltfile")))
	require.NoError(t, err)
	assert.Equal(t, contents, string(bytes))
}

func TestResolveURI(t *testing.T) {
	f := newFixture(t)
	f.MkdirAll("tilt-extensions/hello")
	contents := `hello = "Hi"`
	tiltfile := f.WriteFile("tilt-extensions/hello/Tiltfile", contents)

	path, err := f.finder.resolveURI(uri.URI("ext://hello"))
	require.NoError(t, err)
	assert.Equal(t, tiltfile, path)

	path, err = f.finder.resolveURI(uri.File(tiltfile))
	require.NoError(t, err)
	assert.Equal(t, tiltfile, path)
}

type fixture struct {
	tempdir.TempDirFixture
	finder *extensionFinder
}

func newFixture(t *testing.T) *fixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	f := &fixture{
		TempDirFixture: *tempdir.NewTempDirFixture(t),
		finder:         &extensionFinder{ctx: ctx},
	}
	extPlugin := tiltextension.NewFakePlugin(
		tiltextension.NewFakeExtRepoReconciler(f.Path()),
		tiltextension.NewFakeExtReconciler(f.Path()))
	f.finder.initializePlugins(extPlugin)
	return f
}
