package tiltextension

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestFetchableAlreadyPresentWorks(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://fetchable", "printFoo")
printFoo()
`)
	f.writeModuleLocally("fetchable", libText)

	f.assertExecOutput("foo")
}

func TestUnfetchableAlreadyPresentWorks(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://unfetchable", "printFoo")
printFoo()
`)
	f.writeModuleLocally("unfetchable", libText)

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

func TestExtensionCantIncludeExtension(t *testing.T) {
	f := newExtensionFixture(t)
	defer f.tearDown()

	f.tiltfile(`
load("ext://fetchable", "printFoo")
printFoo()
`)
	f.writeModuleLocally("fetchable", extensionThatLoadsExtension)

	f.assertError("cannot load ext://unfetchable: extensions cannot be loaded from `load`ed Tiltfiles")
}

type extensionFixture struct {
	t   *testing.T
	skf *starkit.Fixture
	tmp *tempdir.TempDirFixture
}

func newExtensionFixture(t *testing.T) *extensionFixture {
	tmp := tempdir.NewTempDirFixture(t)
	ext := NewExtension(
		&fakeFetcher{},
		NewLocalStore(tmp.JoinPath("project")),
	)
	skf := starkit.NewFixture(t, ext)
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

const libText = `
def printFoo():
  print("foo")
`

const extensionThatLoadsExtension = `
load("ext://unfetchable", "printBar")

def printFoo():
	print("foo")
	printBar()
`

type fakeFetcher struct{}

func (f *fakeFetcher) Fetch(ctx context.Context, moduleName string) (ModuleContents, error) {
	if moduleName != "fetchable" {
		return ModuleContents{}, fmt.Errorf("module %s can't be fetched because... reasons", moduleName)
	}
	return ModuleContents{
		Name:             "fetchable",
		TiltfileContents: libText,
	}, nil
}
