package tiltfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestTestFnDeprecated(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
test("test", "echo hi")
`)
	f.loadAssertWarnings(testDeprecationMsg)
}

func TestLocalResourceOnlyUpdateCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", "echo hi", deps=["foo/bar", "foo/a.txt"])
`)

	f.setupFoo()
	f.file(".gitignore", "*.txt")
	f.load()

	f.assertNumManifests(1)
	path1 := "foo/bar"
	path2 := "foo/a.txt"
	m := f.assertNextManifest("test", localTarget(updateCmd(f.Path(), "echo hi", nil), deps(path1, path2)), fileChangeMatches("foo/a.txt"))

	lt := m.LocalTarget()
	assert.Equal(t, []v1alpha1.IgnoreDef{
		{BasePath: f.JoinPath(".git")},
	}, lt.GetFileWatchIgnores())

	f.assertConfigFiles("Tiltfile", ".tiltignore")
}

func TestLocalResourceOnlyServeCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", serve_cmd="sleep 1000")
`)

	f.load()

	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(serveCmd(f.Path(), "sleep 1000", nil)))

	f.assertConfigFiles("Tiltfile", ".tiltignore")
}

func TestLocalResourceUpdateAndServeCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", cmd="echo hi", serve_cmd="sleep 1000")
`)

	f.load()

	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.Path(), "echo hi", nil),
		serveCmd(f.Path(), "sleep 1000", nil),
	))

	f.assertConfigFiles("Tiltfile", ".tiltignore")
}

func TestLocalResourceNeitherUpdateOrServeCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test")
`)

	f.loadErrString("local_resource must have a cmd and/or a serve_cmd, but both were empty")
}

func TestLocalResourceUpdateCmdArray(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", ["echo", "hi"])
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(updateCmdArray(f.Path(), []string{"echo", "hi"}, nil)))
}

func TestLocalResourceServeCmdArray(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", serve_cmd=["echo", "hi"])
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(serveCmdArray(f.Path(), []string{"echo", "hi"}, nil)))
}

func TestLocalResourceWorkdir(t *testing.T) {
	f := newFixture(t)

	f.file("nested/Tiltfile", `
local_resource("nested-local", "echo nested", deps=["foo/bar", "more_nested/repo"])
`)
	f.file("Tiltfile", `
include('nested/Tiltfile')
local_resource("toplvl-local", "echo hello world", deps=["foo/baz", "foo/a.txt"])
`)

	f.setupFoo()
	f.MkdirAll("nested/.git")
	f.MkdirAll("nested/more_nested/repo/.git")
	f.MkdirAll("foo/baz/.git")
	f.MkdirAll("foo/.git") // no Tiltfile lives here, nor is it a LocalResource dep; won't be pulled in as a repo
	f.load()

	f.assertNumManifests(2)
	mNested := f.assertNextManifest("nested-local",
		localTarget(updateCmd(f.JoinPath("nested"), "echo nested", nil),
			deps("nested/foo/bar", "nested/more_nested/repo")))

	ltNested := mNested.LocalTarget()
	assert.Equal(t, []v1alpha1.IgnoreDef{
		{BasePath: f.JoinPath("nested/more_nested/repo", ".git")},
		{BasePath: f.JoinPath("nested", ".git")},
	}, ltNested.GetFileWatchIgnores())

	mTop := f.assertNextManifest("toplvl-local", localTarget(updateCmd(f.Path(), "echo hello world", nil), deps("foo/baz", "foo/a.txt")))
	ltTop := mTop.LocalTarget()
	assert.Equal(t, []v1alpha1.IgnoreDef{
		{BasePath: f.JoinPath("foo/baz", ".git")},
		{BasePath: f.JoinPath(".git")},
	}, ltTop.GetFileWatchIgnores())
}

func TestLocalResourceIgnore(t *testing.T) {
	f := newFixture(t)

	f.file(".dockerignore", "**/**.c")
	f.file("Tiltfile", "include('proj/Tiltfile')")
	f.file("proj/Tiltfile", `
local_resource("test", "echo hi", deps=["foo"], ignore=["**/*.a", "foo/bar.d"])
`)

	f.setupFoo()
	f.file(".gitignore", "*.txt")
	f.load()

	m := f.assertNextManifest("test")

	// TODO(dmiller): I can't figure out how to translate these in to (file\build)(Matches\Filters) assert functions
	filter := ignore.CreateFileChangeFilter(m.LocalTarget().GetFileWatchIgnores())

	for _, tc := range []struct {
		path        string
		expectMatch bool
	}{
		{"proj/foo/bar.a", true},
		{"proj/foo/bar.b", false},
		{"proj/foo/baz/bar.a", true},
		{"proj/foo/bar.d", true},
	} {
		matches, err := filter.Matches(f.JoinPath(tc.path))
		require.NoError(t, err)
		require.Equal(t, tc.expectMatch, matches, tc.path)
	}
}

func TestLocalResourceUpdateCmdEnv(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", "echo hi", env={"KEY1": "value1", "KEY2": "value2"}, serve_cmd="sleep 1000")
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.Path(), "echo hi", []string{"KEY1=value1", "KEY2=value2"}),
		serveCmd(f.Path(), "sleep 1000", nil),
	))
}

func TestLocalResourceServeCmdEnv(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", "echo hi", serve_cmd="sleep 1000", serve_env={"KEY1": "value1", "KEY2": "value2"})
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.Path(), "echo hi", nil),
		serveCmd(f.Path(), "sleep 1000", []string{"KEY1=value1", "KEY2=value2"}),
	))
}

func TestLocalResourceUpdateCmdDir(t *testing.T) {
	f := newFixture(t)

	f.file("nested/inside.txt", "inside the nested directory")
	f.file("Tiltfile", `
local_resource("test", cmd="cat inside.txt", dir="nested")
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.JoinPath(f.Path(), "nested"), "cat inside.txt", nil),
	))
}

func TestLocalResourceUpdateCmdDirNone(t *testing.T) {
	f := newFixture(t)

	f.file("here.txt", "same level")
	f.file("Tiltfile", `
local_resource("test", cmd="cat here.txt", dir=None)
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		updateCmd(f.Path(), "cat here.txt", nil),
	))
}

func TestLocalResourceServeCmdDir(t *testing.T) {
	f := newFixture(t)

	f.file("nested/inside.txt", "inside the nested directory")
	f.file("Tiltfile", `
local_resource("test", serve_cmd="cat inside.txt", serve_dir="nested")
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		serveCmd(f.JoinPath(f.Path(), "nested"), "cat inside.txt", nil),
	))
}

func TestLocalResourceServeCmdDirNone(t *testing.T) {
	f := newFixture(t)

	f.file("here.txt", "same level")
	f.file("Tiltfile", `
local_resource("test", serve_cmd="cat here.txt", serve_dir=None)
`)

	f.load()
	f.assertNumManifests(1)
	f.assertNextManifest("test", localTarget(
		serveCmd(f.Path(), "cat here.txt", nil),
	))
}

func TestLocalResourceStdinModePty(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `
local_resource("test", "echo terminal:$(tty)", stdin_mode="pty")
`)
	f.load()

	f.assertNumManifests(1)
	m := f.assertNextManifest("test")
	lt := m.LocalTarget()
	assert.Equal(t, v1alpha1.StdinModePty, lt.UpdateCmdSpec.StdinMode)
}

func TestLocalResourceStdinModeInvalid(t *testing.T) {
	f := newFixture(t)
	f.file("Tiltfile", `
local_resource("test", "echo terminal:$(tty)", stdin_mode="garbage")
`)
	f.loadErrString("XXX")
}
