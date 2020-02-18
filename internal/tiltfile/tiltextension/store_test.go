package tiltextension

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestWrite(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	path := f.writeModule(ModuleContents{
		Name:              "test",
		TiltfileContents:  "print('hi')",
		GitCommitHash:     "aaaaaa",
		ExtensionRegistry: "https://github.com/windmill/tilt-extensions",
	})

	f.assertPath(path)
	f.assertExtension("test", "print('hi')", "aaaaaa", "https://github.com/windmill/tilt-extensions")
}

func TestWriteAndStat(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.writeModule(ModuleContents{
		Name:              "test",
		TiltfileContents:  "print('hi')",
		GitCommitHash:     "aaaaaa",
		ExtensionRegistry: "https://github.com/windmill/tilt-extensions",
	})

	f.assertExtension("test", "print('hi')", "aaaaaa", "https://github.com/windmill/tilt-extensions")

	path := f.modulePath("test")
	f.assertPath(path)
}

func TestStatModuleDoesntExist(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.assertModulePathDoesntExist("test")
}

func TestTwoExtensions(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.writeModule(ModuleContents{
		Name:              "test1",
		TiltfileContents:  "print('hi')",
		GitCommitHash:     "aaaaaa",
		ExtensionRegistry: "https://github.com/windmill/tilt-extensions",
	})

	f.assertExtension("test1", "print('hi')", "aaaaaa", "https://github.com/windmill/tilt-extensions")

	f.writeModule(ModuleContents{
		Name:              "test2",
		TiltfileContents:  "print('hi')",
		GitCommitHash:     "aaaaaa",
		ExtensionRegistry: "https://github.com/windmill/tilt-extensions",
	})

	f.assertExtension("test2", "print('hi')", "aaaaaa", "https://github.com/windmill/tilt-extensions")
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

func (f *fixture) modulePath(moduleName string) string {
	path, err := f.store.ModulePath(f.ctx, moduleName)
	require.NoError(f.t, err)

	return path
}

func (f *fixture) assertModulePathDoesntExist(moduleName string) {
	path, err := f.store.ModulePath(f.ctx, moduleName)
	assert.True(f.t, os.IsNotExist(err))
	assert.Equal(f.t, "", path)
}

func (f *fixture) assertExtension(moduleName, contents, hash, source string) {
	tiltfileContents, err := ioutil.ReadFile(f.tempdir.JoinPath(extensionDirName, moduleName, extensionFileName))
	require.NoError(f.t, err)

	assert.Equal(f.t, contents, string(tiltfileContents))
	b, err := ioutil.ReadFile(f.tempdir.JoinPath(extensionDirName, metadataFileName))
	require.NoError(f.t, err)

	var mf MetadataFile
	err = json.Unmarshal(b, &mf)
	require.NoError(f.t, err)

	foundModule := false
	for _, e := range mf.Extensions {
		if e.Name == moduleName {
			if foundModule == true {
				f.t.Fatalf("Two modules named %s found in %+v", moduleName, mf)
			}
			foundModule = true
			assert.Equal(f.t, hash, e.GitCommitHash)
			assert.Equal(f.t, source, e.ExtensionRegistry)
		}
	}

	if !foundModule {
		f.t.Errorf("Unable to find module %s in extension metadata file", moduleName)
	}
}

func (f *fixture) assertPath(path string) {
	assert.Equal(f.t, f.tempdir.JoinPath("tilt_modules", "test", extensionFileName), path)
}

func (f *fixture) tearDown() {
	f.tempdir.TearDown()
}
