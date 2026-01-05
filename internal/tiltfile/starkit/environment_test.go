package starkit

import (
	"fmt"
	"path/filepath"
	"strings"
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
		assert.Contains(t, backtrace, fmt.Sprintf("%s:2:1: in <toplevel>", f.JoinPath("Tiltfile")))
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

func NewPluginWithIdentifier(id string) *TestPlugin {
	return &TestPlugin{identifier: id, callCount: 0}
}

type TestPlugin struct {
	identifier string

	// Generally, plugins shouldn't store state this way.
	// They should store it on the Thread object with SetState and friends.
	// But this is OK for testing.
	callCount int
}

func (te *TestPlugin) OnStart(e *Environment) error {
	return e.AddBuiltin(te.identifier, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, err error) {
		te.callCount++
		return starlark.None, nil
	})
}

func TestTopLevelBuiltin(t *testing.T) {
	e := NewPluginWithIdentifier("hi")
	f := NewFixture(t, e)
	f.File("Tiltfile", "hi()")
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, 1, e.callCount)
}

func TestWhile(t *testing.T) {
	e := NewPluginWithIdentifier("hi")
	f := NewFixture(t, e)
	f.File("Tiltfile", `
while True:
  hi()
  break
`)
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, 1, e.callCount)
}

func TestModuleBuiltin(t *testing.T) {
	e := NewPluginWithIdentifier("oh.hai")
	f := NewFixture(t, e)
	f.File("Tiltfile", "oh.hai()")
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, 1, e.callCount)
}

func TestNestedModuleBuiltin(t *testing.T) {
	e := NewPluginWithIdentifier("oh.hai.cat")
	f := NewFixture(t, e)
	f.File("Tiltfile", "oh.hai.cat()")
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, 1, e.callCount)
}

func TestDuplicateGlobalName(t *testing.T) {
	e1 := NewPluginWithIdentifier("foo")
	e2 := NewPluginWithIdentifier("foo")
	f := NewFixture(t, e1, e2)
	f.File("Tiltfile", "foo()")

	_, err := f.ExecFile("Tiltfile")
	require.Errorf(t, err, "Tiltfile exec should fail")
	require.Contains(t, err.Error(), "multiple values added named foo")
	require.Contains(t, err.Error(), "internal error: *starkit.TestPlugin")
}

func TestDuplicateNameWithinModule(t *testing.T) {
	e1 := NewPluginWithIdentifier("bar.foo")
	e2 := NewPluginWithIdentifier("bar.foo")
	f := NewFixture(t, e1, e2)
	f.File("Tiltfile", "bar.foo()")

	_, err := f.ExecFile("Tiltfile")
	require.Errorf(t, err, "Tiltfile exec should fail")
	require.Contains(t, err.Error(), "multiple values added named bar.foo")
	require.Contains(t, err.Error(), "internal error: *starkit.TestPlugin")
}

type PwdPlugin struct{}

func (e PwdPlugin) OnStart(env *Environment) error {
	return env.AddBuiltin("pwd", pwd)
}

func pwd(t *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	t.Print(t, CurrentExecPath(t))
	return starlark.None, nil
}

// foo loads bar
// bar defines `hello` and finishes loading
// foo calls `hello`, which prints foo
func TestUsePwdOfCallSiteLoadingTiltfile(t *testing.T) {
	f := NewFixture(t, PwdPlugin{})
	f.File("bar/Tiltfile", `
def hello():
	pwd()
`)
	f.File("foo/Tiltfile", `
load('../bar/Tiltfile', 'hello')
hello()
`)

	_, err := f.ExecFile("foo/Tiltfile")
	require.NoError(t, err)

	path := strings.TrimSpace(f.out.String())
	require.Equal(t, "foo", filepath.Base(filepath.Dir(path)))
}

// foo loads bar
// bar calls pwd while it's loading, which prints bar
func TestUsePwdOfCallSiteLoadedTiltfile(t *testing.T) {
	f := NewFixture(t, PwdPlugin{})
	f.File("bar/Tiltfile", `
def unused():
  pass
pwd()
`)
	f.File("foo/Tiltfile", `
load('../bar/Tiltfile', 'unused')
`)

	_, err := f.ExecFile("foo/Tiltfile")
	require.NoError(t, err)

	path := strings.TrimSpace(f.out.String())
	require.Equal(t, "bar", filepath.Base(filepath.Dir(path)))
}

// Tiltfile loads Tiltfile2
// 1 prints its __file__ and calls a method in 2 to do the same
func TestUseMagicFileVar(t *testing.T) {
	f := NewFixture(t, PwdPlugin{})
	f.File("Tiltfile2", `
def print_mypath():
  print(__file__)
`)
	f.File("Tiltfile", `
load('Tiltfile2', 'print_mypath')
print(__file__)
print_mypath()
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	paths := strings.Split(strings.TrimSpace(f.out.String()), "\n")
	require.Equal(t, "Tiltfile", filepath.Base(paths[0]))
	require.Equal(t, "Tiltfile2", filepath.Base(paths[1]))
}

func TestSupportsSet(t *testing.T) {
	f := NewFixture(t, PwdPlugin{})
	f.File("Tiltfile", `
x = set([1, 2, 1])
print(x)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	assert.Equal(t, "set([1, 2])\n", f.out.String())
}

func TestSupportDictUnion(t *testing.T) {
	f := NewFixture(t, PwdPlugin{})
	f.File("Tiltfile", `
x = {'a': 1} | {'b': 2}
print(x)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	assert.Equal(t, `{"a": 1, "b": 2}
`, f.out.String())
}
