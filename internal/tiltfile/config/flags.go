package config

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/model"
)

const FlagsConfigFileName = "tilt_config.json"

type Settings struct {
	enabledResources []model.ManifestName
	argDef           ArgsDef

	flagsParsed bool
	FlagsState  model.FlagsState
}

type Extension struct {
	FlagsState model.FlagsState
}

func NewExtension(flagsState model.FlagsState) *Extension {
	return &Extension{FlagsState: flagsState}
}

func (e *Extension) NewState() interface{} {
	return Settings{
		argDef: ArgsDef{args: make(map[string]argDef)},
	}
}

var _ starkit.StatefulExtension = &Extension{}

func (e *Extension) OnExec(t *starlark.Thread, path string) error {
	dir := filepath.Dir(path)
	configPath := filepath.Join(dir, FlagsConfigFileName)

	return starkit.SetState(t, func(settings Settings) Settings {
		settings.FlagsState = e.FlagsState
		settings.FlagsState.ConfigPath = configPath
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
		{"config.define_string_list", argDefinitionBuiltin(func() argValue {
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
		settings.flagsParsed = true
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

	err = io.RecordReadFile(thread, settings.FlagsState.ConfigPath)
	if err != nil {
		return starlark.None, err
	}

	ret, mergedArgs, out, err := settings.argDef.parse(settings.FlagsState)
	if out != "" {
		thread.Print(thread, out)
	}
	if err != nil {
		return starlark.None, err
	}

	if mergedArgs {
		fi, err := os.Stat(settings.FlagsState.ConfigPath)
		if err != nil {
			return starlark.None, errors.Wrapf(err, "statting %s", settings.FlagsState.ConfigPath)
		}
		err = starkit.SetState(thread, func(settings Settings) Settings {
			settings.FlagsState.LastArgsWrite = fi.ModTime()
			return settings
		})
		if err != nil {
			return starlark.None, err
		}
	}

	return ret, nil
}
