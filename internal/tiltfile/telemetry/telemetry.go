package telemetry

import (
	"fmt"
	"path/filepath"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Extension struct{}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return model.TelemetrySettings{
		Period: model.DefaultTelemetryPeriod,
	}
}

func (Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("experimental_telemetry_cmd", setTelemetryCmd)
}

func setTelemetryCmd(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var cmdVal, cmdBatVal starlark.Value
	var period value.Duration
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"cmd", &cmdVal,
		"cmd_bat?", &cmdBatVal,
		"period?", &period)
	if err != nil {
		return starlark.None, err
	}

	cmd, err := value.ValueGroupToCmdHelper(cmdVal, cmdBatVal)
	if err != nil {
		return nil, err
	}

	if cmd.Empty() {
		return starlark.None, fmt.Errorf("cmd cannot be empty")
	}

	err = starkit.SetState(thread, func(settings model.TelemetrySettings) (model.TelemetrySettings, error) {
		if len(settings.Cmd.Argv) > 0 {
			return settings, fmt.Errorf("%v called multiple times; already set to %v", fn.Name(), settings.Cmd)
		}

		settings.Cmd = cmd
		settings.Workdir = filepath.Dir(starkit.CurrentExecPath(thread))
		if !period.IsZero() {
			settings.Period = period.AsDuration()
		}

		return settings, nil
	})

	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

var _ starkit.StatefulExtension = Extension{}

func MustState(model starkit.Model) model.TelemetrySettings {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (model.TelemetrySettings, error) {
	var state model.TelemetrySettings
	err := m.Load(&state)
	return state, err
}
