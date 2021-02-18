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

func TestLoadInterceptorAnalyticsInfoCollection(t *testing.T) {
	f := NewFixture(t)
	f.UseRealFS()

	f.temp.WriteFile("Tiltfile", `
load('this_path_does_not_matter', "x")
`)
	f.temp.WriteFile("foo/Tiltfile", `
x = 1
y = x +1
`)

	infos := []AnalyticsInfo{ExtensionsAnalyticsInfo{ExtensionsLoaded: map[string]bool{"foobar": true}}}
	fi := fakeLoadInterceptor{
		analyticsInfos: infos,
	}
	f.SetLoadInterceptor(fi)
	m, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.Equal(t, infos, m.AnalyticsInfos)

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

type fakeLoadInterceptor struct {
	analyticsInfos []AnalyticsInfo
}

func (fakeLoadInterceptor) LocalPath(t *starlark.Thread, path string) (string, error) {
	return "./foo/Tiltfile", nil
}

func (fi fakeLoadInterceptor) AnalyticsInfo() []AnalyticsInfo {
	return fi.analyticsInfos
}

type failLoadInterceptor struct{}

func (failLoadInterceptor) LocalPath(t *starlark.Thread, path string) (string, error) {
	return "", fmt.Errorf("I'm an error look at me!")
}

func (failLoadInterceptor) AnalyticsInfo() []AnalyticsInfo {
	return nil
}

func NewExtensionWithIdentifier(id string) *TestExtension {
	return &TestExtension{identifier: id, callCount: 0}
}

type TestExtension struct {
	identifier string

	// Generally, extensions shouldn't store state this way.
	// They should store it on the Thread object with SetState and friends.
	// But this is OK for testing.
	callCount int
}

func (te *TestExtension) OnStart(e *Environment) error {
	return e.AddBuiltin(te.identifier, func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (value starlark.Value, err error) {
		te.callCount++
		return starlark.None, nil
	})
}

func TestTopLevelBuiltin(t *testing.T) {
	e := NewExtensionWithIdentifier("hi")
	f := NewFixture(t, e)
	f.File("Tiltfile", "hi()")
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, 1, e.callCount)
}

func TestModuleBuiltin(t *testing.T) {
	e := NewExtensionWithIdentifier("oh.hai")
	f := NewFixture(t, e)
	f.File("Tiltfile", "oh.hai()")
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, 1, e.callCount)
}

func TestNestedModuleBuiltin(t *testing.T) {
	e := NewExtensionWithIdentifier("oh.hai.cat")
	f := NewFixture(t, e)
	f.File("Tiltfile", "oh.hai.cat()")
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, 1, e.callCount)
}

func TestDuplicateGlobalName(t *testing.T) {
	e1 := NewExtensionWithIdentifier("foo")
	e2 := NewExtensionWithIdentifier("foo")
	f := NewFixture(t, e1, e2)
	f.File("Tiltfile", "foo()")

	_, err := f.ExecFile("Tiltfile")
	require.Errorf(t, err, "Tiltfile exec should fail")
	require.Contains(t, err.Error(), "multiple values added named foo")
	require.Contains(t, err.Error(), "internal error: *starkit.TestExtension")
}

func TestDuplicateNameWithinModule(t *testing.T) {
	e1 := NewExtensionWithIdentifier("bar.foo")
	e2 := NewExtensionWithIdentifier("bar.foo")
	f := NewFixture(t, e1, e2)
	f.File("Tiltfile", "bar.foo()")

	_, err := f.ExecFile("Tiltfile")
	require.Errorf(t, err, "Tiltfile exec should fail")
	require.Contains(t, err.Error(), "multiple values added named bar.foo")
	require.Contains(t, err.Error(), "internal error: *starkit.TestExtension")
}

type PwdExtension struct{}

func (e PwdExtension) OnStart(env *Environment) error {
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
	f := NewFixture(t, PwdExtension{})
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
	f := NewFixture(t, PwdExtension{})
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
	f := NewFixture(t, PwdExtension{})
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
