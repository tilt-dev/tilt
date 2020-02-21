package io

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/starlarkstruct"
)

func TestReadFile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("foo.txt", "foo")
	f.File("Tiltfile", `
s = read_file('foo.txt')

load('assert.tilt', 'assert')

assert.equals('foo', str(s))
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
}

func TestReadFileDefault(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
s = read_file('dne.txt', 'foo')

load('assert.tilt', 'assert')

assert.equals('foo', str(s))
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
}

func TestReadFileDefaultEmptyString(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
s = read_file('dne.txt', '')

load('assert.tilt', 'assert')

assert.equals('', str(s))
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
}

// make sure we generate an error on invalid type for default even if the file exists
func TestReadFileInvalidDefaultFileExists(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("foo.txt", "foo")
	f.File("Tiltfile", `
s = read_file('foo.txt', 5)
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "default must be starlark.NoneType or starlark.String. got starlark.Int")
}

func TestReadFileMissing(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", `
s = read_file('dne.txt')
`)

	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "dne.txt: no such file or directory")
}

func newFixture(t *testing.T) *starkit.Fixture {
	f := starkit.NewFixture(t, NewExtension(), starlarkstruct.NewExtension())
	f.UseRealFS()
	f.File("assert.tilt", `
def equals(expected, observed):
	if expected != observed:
		fail("expected: '%s'. observed: '%s'" % (expected, observed))

assert = struct(equals=equals)
`)
	return f
}
