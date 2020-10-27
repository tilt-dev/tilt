package loaddynamic

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestLoadDynamicOK(t *testing.T) {
	f := NewFixture(t)

	f.File("Tiltfile", `
stuff = load_dynamic('./foo/Tiltfile')
print('stuff.x=' + str(stuff['x']))
`)
	f.File("foo/Tiltfile", `
x = 1
print('x=' + str(x))
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, "x=1\nstuff.x=1\n", f.PrintOutput())
}

func TestLoadDynamicCachedLoad(t *testing.T) {
	f := NewFixture(t)

	f.File("Tiltfile", `
stuff1 = load_dynamic('./foo/Tiltfile')
print('stuff1.x=' + str(stuff1['x']))
stuff2 = load_dynamic('./foo/Tiltfile')
print('stuff2.x=' + str(stuff2['x']))
`)
	f.File("foo/Tiltfile", `
x = 1
print('x=' + str(x))
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, "x=1\nstuff1.x=1\nstuff2.x=1\n", f.PrintOutput())
}

func TestLoadDynamicFrozen(t *testing.T) {
	f := NewFixture(t)

	f.File("Tiltfile", `
stuff = load_dynamic('./foo/Tiltfile')
stuff['x'] = 2
`)
	f.File("foo/Tiltfile", `
x = 1
print('x=' + str(x))
`)

	_, err := f.ExecFile("Tiltfile")
	if assert.Error(t, err) {
		backtrace := err.(*starlark.EvalError).Backtrace()
		assert.Contains(t, backtrace, fmt.Sprintf("%s:3:6: in <toplevel>", f.JoinPath("Tiltfile")))
		assert.Contains(t, backtrace, "cannot insert into frozen hash table")
	}
}

func TestLoadDynamicError(t *testing.T) {
	f := NewFixture(t)

	f.File("Tiltfile", `
load_dynamic('./foo/Tiltfile')
`)
	f.File("foo/Tiltfile", `
x = 1
y = x // 0
`)

	_, err := f.ExecFile("Tiltfile")
	if assert.Error(t, err) {
		backtrace := err.(*starlark.EvalError).Backtrace()
		assert.Contains(t, backtrace, fmt.Sprintf("%s:2:13: in <toplevel>", f.JoinPath("Tiltfile")))
		assert.Contains(t, backtrace, fmt.Sprintf("%s:3:7: in <toplevel>", f.JoinPath("foo", "Tiltfile")))
	}
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, &LoadDynamicFn{})
}
