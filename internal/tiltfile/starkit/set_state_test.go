package starkit

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestNormal(t *testing.T) {
	f := newSetStateFixture(t)
	f.File("Tiltfile", `
set_setting(3)
`)
	result, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	require.Equal(t, 3, mustState(result).hello)
}

func TestError(t *testing.T) {
	f := newSetStateFixture(t)
	f.File("Tiltfile", `
set_setting(4)
`)
	result, err := f.ExecFile("Tiltfile")
	require.Error(t, err)
	require.Contains(t, err.Error(), "hello == 4!")
	require.Equal(t, 0, mustState(result).hello)
}

type settings struct {
	hello int
}

type testExtension struct {
}

func newTestExtension() testExtension {
	return testExtension{}
}

func (e testExtension) NewState() interface{} {
	return settings{}
}

func (testExtension) OnStart(env *Environment) error {
	return env.AddBuiltin("set_setting", setSetting)
}

func setSetting(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var hello int
	err := UnpackArgs(thread, fn.Name(), args, kwargs, "hello", &hello)
	if err != nil {
		return starlark.None, err
	}

	err = SetState(thread, func(settings settings) (settings, error) {
		settings.hello = hello
		if hello == 4 {
			return settings, errors.New("hello == 4!")
		}
		return settings, nil
	})

	return starlark.None, err
}

var _ StatefulExtension = testExtension{}

func mustState(m Model) settings {
	var state settings
	err := m.Load(&state)
	if err != nil {
		panic(err)
	}
	return state
}

func newSetStateFixture(tb testing.TB) *Fixture {
	return NewFixture(tb, newTestExtension())
}
