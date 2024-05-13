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

	f.tiltfile(fmt.Sprintf(`
v1alpha1.extension_repo(name='default', url='file://%s/my-custom-repo')

load("ext://my-extension", "printFoo")
printFoo()
`, f.tmp.Path()))

	f.tmp.WriteFile(filepath.Join("my-custom-repo", "my-extension", "Tiltfile"), libText)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "my-extension")
}

func TestLoadedExtensionTwiceDifferentFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		// We don't want to have to bother with file:// escaping on windows.
		// The repo reconciler already tests this.
		t.Skip()
	}

	f := newExtensionFixture(t)

	f.tmp.WriteFile(filepath.Join("my-custom-repo", "my-custom-path", "Tiltfile"), libText)

	subfileContent := fmt.Sprintf(`
v1alpha1.extension_repo(name='my-extension-repo', url='file://%s/my-custom-repo')
v1alpha1.extension(name='my-extension', repo_name='my-extension-repo', repo_path='my-custom-path')
load('ext://my-extension', 'printFoo')
printFoo()
`, f.tmp.Path())

	f.skf.File("Tiltfile.a", subfileContent)
	f.skf.File("Tiltfile.b", subfileContent)
	f.tiltfile(`
include('Tiltfile.a')
include('Tiltfile.b')
`)
	res := f.assertExecOutput("foo\nfoo")
	f.assertLoadRecorded(res, "my-extension")
}

func TestNestingDefaultBehavior(t *testing.T) {
	if runtime.GOOS == "windows" {
		// We don't want to have to bother with file:// escaping on windows.
		// The repo reconciler already tests this.
		t.Skip()
	}

	// The default behavior of the extension loading mechanism converts slashes in extension names
	// to an _, but retains the original extension name as the path within the extension repository.
	// You can leverage this for nested extensions by defining an extension with an underscore and
	// then loading it with a slash.
	f := newExtensionFixture(t)

	f.tiltfile(fmt.Sprintf(`
v1alpha1.extension_repo(name='custom', url='file://%s/my-custom-repo')
v1alpha1.extension(name='nested_fake', repo_name='custom', repo_path='fake')
v1alpha1.extension(name='nested_real', repo_name='custom', repo_path='nested/real')

load("ext://nested/fake", "printFake")
printFake()

load("ext://nested/real", "printReal")
printReal()
`, f.tmp.Path()))

	fakeContent := `
def printFake():
    print("fake")
	`

	realContent := `
def printReal():
	print("real")
	`

	f.tmp.WriteFile(filepath.Join("my-custom-repo", "fake", "Tiltfile"), fakeContent)
	f.tmp.WriteFile(filepath.Join("my-custom-repo", "nested", "real", "Tiltfile"), realContent)

	res := f.assertExecOutput("fake\nreal")
	f.assertLoadRecorded(res, "nested/fake", "nested/real")
}

func TestRepoPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		// We don't want to have to bother with file:// escaping on windows.
		// The repo reconciler already tests this.
		t.Skip()
	}

	// Assert that extension repositories with a prefix allow "autoregistration" of extensions
	// if the extension prefix starts with the registered repository prefix.
	f := newExtensionFixture(t)

	f.tiltfile(fmt.Sprintf(`
v1alpha1.extension_repo(name='custom', url='file://%s/ext-repo', prefix='custom')

load("ext://custom/ext", "printFoo")
printFoo()
`, f.tmp.Path()))

	f.tmp.WriteFile(filepath.Join("ext-repo", "ext", "Tiltfile"), libText)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "custom/ext")
}

func TestRepoPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		// We don't want to have to bother with file:// escaping on windows.
		// The repo reconciler already tests this.
		t.Skip()
	}

	// Assert that extension repositories with a defined subpath load registered extensions
	// from that subpath
	f := newExtensionFixture(t)

	f.tiltfile(fmt.Sprintf(`
v1alpha1.extension_repo(name='custom', url='file://%s/ext-repo', path='subdir')
v1alpha1.extension(name='my-ext', repo_name='custom')
v1alpha1.extension(name='my-ext-with-path', repo_name='custom', repo_path='subdir2')

# Assert that loading an extension without a repo_path loads from the repo-wide path
load("ext://my-ext", "printExt")
printExt()

load("ext://my-ext-with-path", "printExt2")
printExt2()
`, f.tmp.Path()))

	extContent := `
def printExt():
	print("main ext")
	`

	extContent2 := `
def printExt2():
	print("sub ext")
	`

	f.tmp.WriteFile(filepath.Join("ext-repo", "subdir", "Tiltfile"), extContent)
	f.tmp.WriteFile(filepath.Join("ext-repo", "subdir", "subdir2", "Tiltfile"), extContent2)

	res := f.assertExecOutput("main ext\nsub ext")
	f.assertLoadRecorded(res, "my-ext", "my-ext-with-path")
}

func TestRepoPrefixAndPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		// We don't want to have to bother with file:// escaping on windows.
		// The repo reconciler already tests this.
		t.Skip()
	}

	// Assert that extension repositories with a defined subpath load registered extensions
	// from that subpath, including autoregistration by prefix match
	f := newExtensionFixture(t)

	f.tiltfile(fmt.Sprintf(`
v1alpha1.extension_repo(name='custom', url='file://%s/ext-repo', prefix='custom', path='subdir')

# Should load an extension from the custom repo at <repo.path>/my-ext
load("ext://custom/my-ext", "printExt")
printExt()

# Should load from <repo.path>/my-ext/subext
load("ext://custom/my-ext/subext", "printSub")
printSub()
`, f.tmp.Path()))

	extContent := `
def printExt():
	print("main ext")
	`

	subExtContent := `
def printSub():
	print("sub ext")
	`

	f.tmp.WriteFile(filepath.Join("ext-repo", "subdir", "my-ext", "Tiltfile"), extContent)
	f.tmp.WriteFile(filepath.Join("ext-repo", "subdir", "my-ext", "subext", "Tiltfile"), subExtContent)

	res := f.assertExecOutput("main ext\nsub ext")
	f.assertLoadRecorded(res, "custom/my-ext", "custom/my-ext/subext")
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
