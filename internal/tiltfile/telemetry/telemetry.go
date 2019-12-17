package telemetry

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/model"
)

type Settings struct {
	Cmd model.Cmd
}

type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return Settings{
		Cmd: model.Cmd{},
	}
}

func (Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("experimental_telemetry_cmd", setTelemetryCmd)
}

func setTelemetryCmd(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var cmd string
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "cmd", &cmd)
	if err != nil {
		return starlark.None, err
	}

	if len(cmd) == 0 {
		return starlark.None, fmt.Errorf("cmd cannot be empty")
	}

	var innerErr error
	err = starkit.SetState(thread, func(settings Settings) Settings {
		if len(settings.Cmd.Argv) > 0 {
			innerErr = fmt.Errorf("%v called multiple times; already set to %v", fn.Name(), settings.Cmd)
			return settings
		}

		settings.Cmd = model.ToShellCmd(cmd)

		return settings
	})

	if err != nil {
		return starlark.None, err
	}
	if innerErr != nil {
		return starlark.None, innerErr
	}

	return starlark.None, nil
}

var _ starkit.StatefulExtension = Extension{}

func MustState(model starkit.Model) Settings {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (Settings, error) {
	var state Settings
	err := m.Load(&state)
	return state, err
}
