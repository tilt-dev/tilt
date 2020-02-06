package extension

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

func TestWrite(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	path := f.writeModule(ModuleContents{
		Name:             "test",
		TiltfileContents: "print('hi')",
		GitCommitHash:    "aaaaaa",
		Source:           "https://github.com/windmill/extensions",
	})

	f.assertPath(path)
	f.assertExtension("test", "print('hi')", "aaaaaa", "https://github.com/windmill/extensions")
}

func TestWriteAndStat(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.writeModule(ModuleContents{
		Name:             "test",
		TiltfileContents: "print('hi')",
		GitCommitHash:    "aaaaaa",
		Source:           "https://github.com/windmill/extensions",
	})

	f.assertExtension("test", "print('hi')", "aaaaaa", "https://github.com/windmill/extensions")

	path := f.stat("test")
	f.assertPath(path)
}

func TestStatModuleDoesntExist(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	assert.Equal(f.t, "", f.stat("test"))
}

type fixture struct {
	t       *testing.T
	ctx     context.Context
	tempdir *tempdir.TempDirFixture
	store   *LocalStore
}

func newFixture(t *testing.T) *fixture {
	ctx := context.Background()
	temp := tempdir.NewTempDirFixture(t)
	ls := NewLocalStore(temp.Path())

	return &fixture{
		t:       t,
		ctx:     ctx,
		tempdir: temp,
		store:   ls,
	}
}

func (f *fixture) writeModule(contents ModuleContents) string {
	path, err := f.store.Write(f.ctx, contents)
	require.NoError(f.t, err)

	return path
}

func (f *fixture) stat(moduleName string) string {
	path, err := f.store.Stat(f.ctx, moduleName)
	require.NoError(f.t, err)

	return path
}

func (f *fixture) assertExtension(moduleName, contents, hash, source string) {
	tiltfileContents, err := ioutil.ReadFile(f.tempdir.JoinPath(extensionDirName, moduleName, tiltfile.FileName))
	require.NoError(f.t, err)

	assert.Equal(f.t, contents, string(tiltfileContents))
	// TODO(dmiller): assert on hash and source
}

func (f *fixture) assertPath(path string) {
	assert.Equal(f.t, f.tempdir.JoinPath("tilt_modules", "test", "Tiltfile"), path)
}

func (f *fixture) tearDown() {
	f.tempdir.TearDown()
}
