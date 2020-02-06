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

func NewExtensionWithIdentifier(id string) Extension {
	return TestExtension{id}
}

type TestExtension struct {
	identifier string
}

func (te TestExtension) OnStart(e *Environment) error {
	return e.AddBuiltin(te.identifier, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, err error) {
		return starlark.None, nil
	})
}

func TestDuplicateGlobalName(t *testing.T) {
	e1 := NewExtensionWithIdentifier("foo")
	e2 := NewExtensionWithIdentifier("foo")
	f := NewFixture(t, e1, e2)
	f.File("Tiltfile", "foo()")

	_, err := f.ExecFile("Tiltfile")
	require.Errorf(t, err, "Tiltfile exec should fail")
	require.Contains(t, err.Error(), "multiple values added named foo")
	require.Contains(t, err.Error(), "internal error: starkit.TestExtension")
}

func TestDuplicateNameWithinModule(t *testing.T) {
	e1 := NewExtensionWithIdentifier("bar.foo")
	e2 := NewExtensionWithIdentifier("bar.foo")
	f := NewFixture(t, e1, e2)
	f.File("Tiltfile", "bar.foo()")

	_, err := f.ExecFile("Tiltfile")
	require.Errorf(t, err, "Tiltfile exec should fail")
	require.Contains(t, err.Error(), "multiple values added named bar.foo")
	require.Contains(t, err.Error(), "internal error: starkit.TestExtension")
}
