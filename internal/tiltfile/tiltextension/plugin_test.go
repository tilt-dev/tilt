package tiltextension

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile/include"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	tiltfilev1alpha1 "github.com/tilt-dev/tilt/internal/tiltfile/v1alpha1"
)

func TestFetchableAlreadyPresentWorks(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://fetchable", "printFoo")
printFoo()
`)
	f.writeModuleLocally("fetchable", libText)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "fetchable")
}

func TestAlreadyPresentWorks(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://unfetchable", "printFoo")
printFoo()
`)
	f.writeModuleLocally("unfetchable", libText)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "unfetchable")
}

func TestExtensionRepoApplyFails(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://module", "printFoo")
printFoo()
`)
	f.extrr.Error = "repo can't be fetched"

	res := f.assertError("loading extension repo default: repo can't be fetched")
	f.assertNoLoadsRecorded(res)
}

func TestExtensionApplyFails(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://module", "printFoo")
printFoo()
`)
	f.extr.Error = "ext can't be fetched"

	res := f.assertError("loading extension module: ext can't be fetched")
	f.assertNoLoadsRecorded(res)
}

func TestIncludedFileMayIncludeExtension(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`include('Tiltfile.prime')`)

	f.skf.File("Tiltfile.prime", `
load("ext://fetchable", "printFoo")
printFoo()
`)

	f.writeModuleLocally("fetchable", libText)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "fetchable")
}

func TestExtensionMayLoadExtension(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://fooExt", "printFoo")
printFoo()
`)
	f.writeModuleLocally("fooExt", extensionThatLoadsExtension)
	f.writeModuleLocally("barExt", printBar)

	res := f.assertExecOutput("foo\nbar")
	f.assertLoadRecorded(res, "fooExt", "barExt")
}

func TestLoadedFilesResolveExtensionsFromRootTiltfile(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`include('./nested/Tiltfile')`)

	f.tmp.MkdirAll("nested")
	f.skf.File("nested/Tiltfile", `
load("ext://unfetchable", "printFoo")
printFoo()
`)

	// Note that the extension lives in the tilt_modules directory of the
	// root Tiltfile. (If we look for this extension in the wrong place and
	// try to fetch this extension into ./nested/tilt_modules,
	// the fake fetcher will error.)
	f.writeModuleLocally("unfetchable", libText)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "unfetchable")
}

func TestRepoAndExtOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		// We don't want to have to bother with file:// escaping on windows.
		// The repo reconciler already tests this.
		t.Skip()
	}

	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(fmt.Sprintf(`
v1alpha1.extension_repo(name='default', url='file://%s/my-custom-repo')
v1alpha1.extension(name='my-extension', repo_name='default', repo_path='my-custom-path')

load("ext://my-extension", "printFoo")
printFoo()
`, f.tmp.Path()))

	f.tmp.WriteFile(filepath.Join("my-custom-repo", "my-custom-path", "Tiltfile"), libText)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "my-extension")
}

func TestRepoOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		// We don't want to have to bother with file:// escaping on windows.
		// The repo reconciler already tests this.
		t.Skip()
	}

	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(fmt.Sprintf(`
v1alpha1.extension_repo(name='default', url='file://%s/my-custom-repo')

load("ext://my-extension", "printFoo")
printFoo()
`, f.tmp.Path()))

	f.tmp.WriteFile(filepath.Join("my-custom-repo", "my-extension", "Tiltfile"), libText)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "my-extension")
}

type extensionFixture struct {
	t     *testing.T
	skf   *starkit.Fixture
	tmp   *tempdir.TempDirFixture
	extr  *FakeExtReconciler
	extrr *FakeExtRepoReconciler
}

func newExtensionFixture(t *testing.T) *extensionFixture {
	tmp := tempdir.NewTempDirFixture(t)
	extr := NewFakeExtReconciler(tmp.Path())
	extrr := NewFakeExtRepoReconciler(tmp.Path())

	ext := NewFakePlugin(
		extrr,
		extr,
	)
	skf := starkit.NewFixture(t, ext, include.IncludeFn{}, tiltfilev1alpha1.NewPlugin())
	skf.UseRealFS()

	return &extensionFixture{
		t:     t,
		skf:   skf,
		tmp:   tmp,
		extr:  extr,
		extrr: extrr,
	}
}

func (f *extensionFixture) tearDown() {
	defer f.tmp.TearDown()
	defer f.skf.TearDown()
}

func (f *extensionFixture) tiltfile(contents string) {
	f.skf.File("Tiltfile", contents)
}

func (f *extensionFixture) assertExecOutput(expected string) starkit.Model {
	result, err := f.skf.ExecFile("Tiltfile")
	if err != nil {
		f.t.Fatalf("unexpected error %v", err)
	}
	if !strings.Contains(f.skf.PrintOutput(), expected) {
		f.t.Fatalf("output %q doesn't contain expected output %q", f.skf.PrintOutput(), expected)
	}
	return result
}

func (f *extensionFixture) assertError(expected string) starkit.Model {
	result, err := f.skf.ExecFile("Tiltfile")
	if err == nil {
		f.t.Fatalf("expected error; got none (output %q)", f.skf.PrintOutput())
	}
	if !strings.Contains(err.Error(), expected) {
		f.t.Fatalf("error %v doesn't contain expected text %q", err, expected)
	}
	return result
}

func (f *extensionFixture) assertLoadRecorded(model starkit.Model, expected ...string) {
	state := MustState(model)

	expectedSet := map[string]bool{}
	for _, exp := range expected {
		expectedSet[exp] = true
	}

	assert.Equal(f.t, expectedSet, state.ExtsLoaded)
}

func (f *extensionFixture) assertNoLoadsRecorded(model starkit.Model) {
	f.assertLoadRecorded(model)
}

func (f *extensionFixture) writeModuleLocally(name string, contents string) {
	f.tmp.WriteFile(filepath.Join("tilt-extensions", name, "Tiltfile"), contents)
}

const libText = `
def printFoo():
  print("foo")
`

const printBar = `
def printBar():
  print("bar")
`

const extensionThatLoadsExtension = `
load("ext://barExt", "printBar")

def printFoo():
	print("foo")
	printBar()
`
