package tiltfile

import (
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

	f.loadErrString("Circular tiltfile load")
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
