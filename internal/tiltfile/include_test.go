package tiltfile

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncludeThreeTiltfiles(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.setupFooAndBar()
	f.file("foo/Tiltfile", `
docker_build('gcr.io/foo', '.')
`)
	f.file("bar/Tiltfile", `
docker_build('gcr.io/bar', '.')
`)
	f.file("Tiltfile", `
include('./foo/Tiltfile')
include('./bar/Tiltfile')
k8s_yaml(['foo.yaml', 'bar.yaml'])
`)

	f.load()
	f.assertNextManifest("foo",
		db(image("gcr.io/foo")),
		deployment("foo"))
	f.assertNextManifest("bar",
		db(image("gcr.io/bar")),
		deployment("bar"))
}

func TestIncludeCircular(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("foo/Tiltfile", `
include('../Tiltfile')
`)
	f.file("Tiltfile", `
include('./foo/Tiltfile')
`)

	f.loadErrString("Circular load")
}

func TestIncludeTriangular(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("foo/Tiltfile", `
print('foo')
`)
	f.file("bar/Tiltfile", `
include('../foo/Tiltfile')
`)
	f.file("Tiltfile", `
include('./foo/Tiltfile')
include('./bar/Tiltfile')
`)

	f.load()

	// make sure foo/Tiltfile is only loaded once
	assert.Equal(t,
		"Beginning Tiltfile execution\nfoo\nSuccessfully loaded Tiltfile\n",
		f.out.String())
}

func TestIncludeMissing(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
include('./foo/Tiltfile')
`)

	f.loadErrString(
		"Tiltfile:2:8: in <toplevel>",
		"no such file or directory")
}

func TestIncludeError(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
include('./foo/Tiltfile')
`)
	f.file("foo/Tiltfile", `
local('exit 1')
`)

	f.loadErrString(
		fmt.Sprintf("%s/Tiltfile:2:8: in <toplevel>", f.Path()),
		fmt.Sprintf("%s/foo/Tiltfile:2:6: in <toplevel>", f.Path()),
		"exit status 1")
}

func TestLoadFunction(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("boo/Tiltfile", `
def shout():
  print('boo')
`)
	f.file("Tiltfile", `
load('./boo/Tiltfile', 'shout')
shout()
`)

	f.load()
	assert.Equal(t,
		"Beginning Tiltfile execution\nboo\nSuccessfully loaded Tiltfile\n",
		f.out.String())
}

func TestLoadError(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
load('./foo/Tiltfile', "x")
`)
	f.file("foo/Tiltfile", `
x = 1
local('exit 1')
`)

	// TODO(nick): Currently this doesn't include the complete
	// trace, because we need an upstream starlark fix.
	// https://github.com/google/starlark-go/pull/244
	f.loadErrString(
		fmt.Sprintf("%s/Tiltfile:2:1: in <toplevel>", f.Path()),
		"cannot load ./foo/Tiltfile",
		"exit status 1")
}
