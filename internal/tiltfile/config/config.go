package config

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

const UserConfigFileName = "tilt_config.json"

type Settings struct {
	disableAll       bool
	enabledResources []model.ManifestName
	configDef        ConfigDef

	configParseCalled bool

	// if parse has been called, the directory containing the Tiltfile that called it
	seenWorkingDirectory string
}

type Plugin struct {
	tiltSubcommand model.TiltSubcommand
}

func NewPlugin(tiltSubcommand model.TiltSubcommand) *Plugin {
	return &Plugin{tiltSubcommand: tiltSubcommand}
}

func (e *Plugin) NewState() interface{} {
	return Settings{
		configDef: ConfigDef{configSettings: make(map[string]configSetting)},
	}
}

var _ starkit.StatefulPlugin = &Plugin{}

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

func (e *Plugin) OnStart(env *starkit.Environment) error {
	for _, b := range []struct {
		name string
		f    starkit.Function
	}{
		{"config.set_enabled_resources", setEnabledResources},
		{"config.parse", e.parse},
		{"config.define_string_list", configSettingDefinitionBuiltin(func() configValue {
			return &stringList{}
		})},
		{"config.define_string", configSettingDefinitionBuiltin(func() configValue {
			return &stringSetting{}
		})},
		{"config.define_bool", configSettingDefinitionBuiltin(func() configValue {
			return &boolSetting{}
		})},
		{"config.define_object", configSettingDefinitionBuiltin(func() configValue {
			return &objectSetting{}
		})},
	} {
		err := env.AddBuiltin(b.name, b.f)
		if err != nil {
			return errors.Wrap(err, b.name)
		}
	}

	err := env.AddValue("config.tilt_subcommand", starlark.String(e.tiltSubcommand))
	if err != nil {
		return err
	}

	startPath := env.StartTiltfile().Spec.Path
	err = env.AddValue("config.main_path", starlark.String(startPath))
	if err != nil {
		return err
	}

	err = env.AddValue("config.main_dir", starlark.String(filepath.Dir(startPath)))
	if err != nil {
		return err
	}

	return err
}

func (e *Plugin) parse(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs)
	if err != nil {
		return starlark.None, err
	}

	wd := starkit.AbsWorkingDir(thread)

	err = starkit.SetState(thread, func(settings Settings) (Settings, error) {
		if settings.seenWorkingDirectory != "" && settings.seenWorkingDirectory != wd {
			return settings, fmt.Errorf(
				"%s can only be called from one Tiltfile working directory per run. It was called from %s and %s",
				fn.Name(),
				settings.seenWorkingDirectory,
				wd)
		}
		settings.seenWorkingDirectory = wd
		settings.configParseCalled = true
		return settings, nil
	})
	if err != nil {
		return starlark.None, err
	}

	tf, err := starkit.StartTiltfileFromThread(thread)
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

	userConfigPath := filepath.Join(wd, UserConfigFileName)

	err = io.RecordReadPath(thread, io.WatchFileOnly, userConfigPath)
	if err != nil {
		return starlark.None, err
	}

	ret, out, err := settings.configDef.parse(userConfigPath, tf.Spec.Args)
	if out != "" {
		thread.Print(thread, out)
	}
	if err != nil {
		return starlark.None, err
	}

	return ret, nil
}
