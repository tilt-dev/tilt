package tiltfile

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/testutils"
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

	f.assertConfigFiles(".tiltignore", "Tiltfile",
		"bar.yaml", "bar/.dockerignore", "bar/Dockerfile", "bar/Tiltfile",
		"foo.yaml", "foo/.dockerignore", "foo/Dockerfile", "foo/Tiltfile")
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
	assertContainsOnce(
		t,
		f.out.String(),
		"Beginning Tiltfile execution\nfoo\nSuccessfully loaded Tiltfile")
}

func TestIncludeMissing(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
include('./foo/Tiltfile')
`)

	f.loadErrString(
		"Tiltfile:2:8: in <toplevel>",
		testutils.IsNotExistMessage())
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
		fmt.Sprintf("%s:2:8: in <toplevel>", f.JoinPath("Tiltfile")),
		fmt.Sprintf("%s:2:6: in <toplevel>", f.JoinPath("foo", "Tiltfile")),
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
	assertContainsOnce(
		t,
		f.out.String(),
		"Beginning Tiltfile execution\nboo\nSuccessfully loaded Tiltfile")

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

	f.loadErrString(
		fmt.Sprintf("%s:2:1: in <toplevel>", f.JoinPath("Tiltfile")),
		fmt.Sprintf("%s:3:6: in <toplevel>", f.JoinPath("foo", "Tiltfile")),
		"exit status 1")
}

func assertContainsOnce(t *testing.T, s, contains string) {
	assert.Contains(t, s, contains)
	assert.Equal(t, 1, strings.Count(s, contains))
}
