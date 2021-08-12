package tiltextension

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

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
	f.writeModuleLocally("fetchable", libText)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "fetchable")
}

func TestUnfetchableAlreadyPresentWorks(t *testing.T) {
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

func TestFetchFetchableWorks(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://fetchable", "printFoo")
printFoo()
`)

	res := f.assertExecOutput("foo")
	f.assertLoadRecorded(res, "fetchable")
}

func TestFetchUnfetchableFails(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://unfetchable", "printFoo")
printFoo()
`)

	res := f.assertError("unfetchable can't be fetched")
	f.assertNoLoadsRecorded(res) // don't record metrics for failed imports
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

type extensionFixture struct {
	t   *testing.T
	skf *starkit.Fixture
	tmp *tempdir.TempDirFixture
}

func newExtensionFixture(t *testing.T) *extensionFixture {
	tmp := tempdir.NewTempDirFixture(t)
	ext := NewPlugin(
		&fakeFetcher{t: t},
		NewLocalStore(tmp.JoinPath("project")),
	)
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
		f.t.Fatalf("error %v doens't contain expected text %q", err, expected)
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
	f.tmp.WriteFile(filepath.Join("project", "tilt_modules", name, "Tiltfile"), contents)
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

type fakeFetcher struct {
	t *testing.T
}

func (f *fakeFetcher) Fetch(ctx context.Context, moduleName string) (ModuleContents, error) {
	if moduleName != "fetchable" {
		return ModuleContents{}, fmt.Errorf("module %s can't be fetched because... reasons", moduleName)
	}

	return ModuleContents{
		Name: "fetchable",
		Dir:  dirWithTiltfile(f.t, libText),
	}, nil
}

func (f *fakeFetcher) CleanUp() error {
	return nil
}
