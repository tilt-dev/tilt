package config

import (
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/model"
)

const UserConfigFileName = "tilt_config.json"

type Settings struct {
	enabledResources []model.ManifestName
	configDef        ConfigDef

	configPath string

	configParseCalled bool
	UserConfigState   model.UserConfigState
}

type Extension struct {
	UserConfigState model.UserConfigState
}

func NewExtension(userConfigState model.UserConfigState) *Extension {
	return &Extension{UserConfigState: userConfigState}
}

func (e *Extension) NewState() interface{} {
	return Settings{
		configDef: ConfigDef{configSettings: make(map[string]configSetting)},
	}
}

var _ starkit.StatefulExtension = &Extension{}

func (e *Extension) OnExec(t *starlark.Thread, path string) error {
	dir := filepath.Dir(path)
	configPath := filepath.Join(dir, UserConfigFileName)

	return starkit.SetState(t, func(settings Settings) Settings {
		settings.UserConfigState = e.UserConfigState
		settings.configPath = configPath
		return settings
	})
}

var _ starkit.OnExecExtension = &Extension{}

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

func (e *Extension) OnStart(env *starkit.Environment) error {
	for _, b := range []struct {
		name string
		f    starkit.Function
	}{
		{"config.set_enabled_resources", setEnabledResources},
		{"config.parse", e.parse},
		{"config.define_string_list", configSettingDefinitionBuiltin(func() configValue {
			return &stringList{}
		})},
	} {
		err := env.AddBuiltin(b.name, b.f)
		if err != nil {
			return errors.Wrap(err, b.name)
		}
	}

	return nil
}

func (e *Extension) parse(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs)
	if err != nil {
		return starlark.None, err
	}

	err = starkit.SetState(thread, func(settings Settings) Settings {
		settings.configParseCalled = true
		return settings
	})
	if err != nil {
		return starlark.None, err
	}

	m, err := starkit.ModelFromThread(thread)
	if err != nil {
		return starlark.None, err
	}
	settings, err := GetState(m)
	if err != nil {
		return starlark.None, err
	}

	err = io.RecordReadFile(thread, settings.configPath)
	if err != nil {
		return starlark.None, err
	}

	ret, out, err := settings.configDef.parse(settings.configPath, settings.UserConfigState.Args)
	if out != "" {
		thread.Print(thread, out)
	}
	if err != nil {
		return starlark.None, err
	}

	return ret, nil
}
