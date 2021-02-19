package tiltextension

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile/include"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestFetchableAlreadyPresentWorks(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://fetchable", "printFoo")
printFoo()
`)
	f.writeModuleLocally("fetchable", testLibText)

	f.assertExecOutput("foo")
}

func TestUnfetchableAlreadyPresentWorks(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://unfetchable", "printFoo")
printFoo()
`)
	f.writeModuleLocally("unfetchable", testLibText)

	f.assertExecOutput("foo")
}

func TestFetchFetchableWorks(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://fetchable", "printFoo")
printFoo()
`)

	f.assertExecOutput("foo")
}

func TestFetchUnfetchableFails(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://unfetchable", "printFoo")
printFoo()
`)

	f.assertError("unfetchable can't be fetched")
}

func TestIncludedFileMayIncludeExtension(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`include('Tiltfile.prime')`)

	f.skf.File("Tiltfile.prime", `
load("ext://fetchable", "printFoo")
printFoo()
`)

	f.writeModuleLocally("fetchable", testLibText)

	f.assertExecOutput("foo")
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

	f.assertExecOutput("foo\nbar")
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
	f.writeModuleLocally("unfetchable", testLibText)

	f.assertExecOutput("foo")
}

type extensionFixture struct {
	t   *testing.T
	skf *starkit.Fixture
	tmp *tempdir.TempDirFixture
}

func newExtensionFixture(t *testing.T) *extensionFixture {
	tmp := tempdir.NewTempDirFixture(t)
	ext := NewFakeExtension(t, tmp.JoinPath("project"))
	skf := starkit.NewFixture(t, ext, include.IncludeFn{})
	skf.UseRealFS()

	return &extensionFixture{
		t:   t,
		skf: skf,
		tmp: tmp,
	}
}

func (f *extensionFixture) tearDown() {
	defer f.tmp.TearDown()
	defer f.skf.TearDown()
}

func (f *extensionFixture) tiltfile(contents string) {
	f.skf.File("Tiltfile", contents)
}

func (f *extensionFixture) assertExecOutput(expected string) {
	_, err := f.skf.ExecFile("Tiltfile")
	if err != nil {
		f.t.Fatalf("unexpected error %v", err)
	}
	if !strings.Contains(f.skf.PrintOutput(), expected) {
		f.t.Fatalf("output %q doesn't contain expected output %q", f.skf.PrintOutput(), expected)
	}
}

func (f *extensionFixture) assertError(expected string) {
	_, err := f.skf.ExecFile("Tiltfile")
	if err == nil {
		f.t.Fatalf("expected error; got none (output %q)", f.skf.PrintOutput())
	}
	if !strings.Contains(err.Error(), expected) {
		f.t.Fatalf("error %v doens't contain expected text %q", err, expected)
	}
}

func (f *extensionFixture) writeModuleLocally(name string, contents string) {
	f.tmp.WriteFile(filepath.Join("project", "tilt_modules", name, "Tiltfile"), contents)
}

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
