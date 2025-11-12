package tiltfile

import "testing"

func TestTestFnDeprecated(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
test("test", "echo hi")
`)
	f.loadAssertWarnings(testDeprecationMsg)
}

func TestLocalResourceDirWithoutCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", serve_cmd="python server.py", dir="./foo")
`)
	f.loadErrString("'dir' only affects 'cmd', not 'serve_cmd'. Did you mean to use 'serve_dir' instead?")
}

func TestLocalResourceDirWithoutCmdNoServe(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", dir="./foo")
`)
	f.loadErrString("'dir' specified but 'cmd' is empty")
}

func TestLocalResourceServeDirWithoutServeCmd(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", cmd="echo hi", serve_dir="./foo")
`)
	f.loadErrString("'serve_dir' specified but 'serve_cmd' is empty")
}

func TestLocalResourceDirWithCmdWorks(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", cmd="echo hi", dir="./foo")
`)
	f.load()
}

func TestLocalResourceServeDirWithServeCmdWorks(t *testing.T) {
	f := newFixture(t)

	f.file("Tiltfile", `
local_resource("test", serve_cmd="python server.py", serve_dir="./foo")
`)
	f.load()
}
