package starkit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	_, err := f.ExecFile("Tiltfile")
	if assert.Error(t, err) {
		backtrace := err.(*starlark.EvalError).Backtrace()
		assert.Contains(t, backtrace, fmt.Sprintf("%s/Tiltfile:2:1: in <toplevel>", f.Path()))
		assert.Contains(t, backtrace, "cannot load ./foo/Tiltfile")
	}
}

func TestLoadInterceptor(t *testing.T) {
	f := NewFixture(t)
	f.UseRealFS()

	f.temp.WriteFile("Tiltfile", `
load('this_path_does_not_matter', "x")
`)
	f.temp.WriteFile("foo/Tiltfile", `
x = 1
y = x +1
`)

	fi := fakeLoadInterceptor{}
	f.SetLoadInterceptor(fi)
	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
}

func TestLoadInterceptorThatFails(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
load('./foo/Tiltfile', "x")
`)
	f.File("foo/Tiltfile", `
x = 1
y = x + 1
`)

	fi := failLoadInterceptor{}
	f.SetLoadInterceptor(fi)
	_, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "I'm an error look at me!")
}

type fakeLoadInterceptor struct{}

func (fakeLoadInterceptor) LocalPath(t *starlark.Thread, path string) (string, error) {
	return "./foo/Tiltfile", nil
}

type failLoadInterceptor struct{}

func (failLoadInterceptor) LocalPath(t *starlark.Thread, path string) (string, error) {
	return "", fmt.Errorf("I'm an error look at me!")
}
