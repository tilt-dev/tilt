package starkit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.starlark.net/starlark"
)

func TestLoadError(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
load('./foo/Tiltfile', "x")
`)
	f.File("foo/Tiltfile", `
x = 1
y = x // 0
`)

	err := f.ExecFile("Tiltfile")
	if assert.Error(t, err) {
		backtrace := err.(*starlark.EvalError).Backtrace()
		assert.Contains(t, backtrace, fmt.Sprintf("%s/Tiltfile:2:1: in <toplevel>", f.Path()))
		assert.Contains(t, backtrace, "cannot load ./foo/Tiltfile")
	}
}
