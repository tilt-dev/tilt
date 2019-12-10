package config

import (
	"bytes"
	"errors"
	"flag"
	"fmt"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type configValue interface {
	flag() flag.Value
	starlark() starlark.Value
	setFromArgs([]string)
}

type configSetting struct {
	configValue
	usage string
}

type ConfigDef struct {
	positionalSettingName string
	configSettings        map[string]configSetting
}

func (cd ConfigDef) parse(args []string) (v starlark.Value, output string, err error) {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	w := &bytes.Buffer{}
	fs.SetOutput(w)

	for name, def := range cd.configSettings {
		if name == cd.positionalSettingName {
			continue
		}
		v := def.flag()
		fs.Var(v, name, def.usage)
	}

	err = fs.Parse(args)
	if err != nil {
		return starlark.None, w.String(), err
	}

	if len(fs.Args()) > 0 {
		if cd.positionalSettingName == "" {
			return starlark.None, w.String(), errors.New("positional args were specified, but none were expected (no setting defined with args=True)")
		} else {
			cd.configSettings[cd.positionalSettingName].setFromArgs(fs.Args())
		}
	}

	ret := starlark.NewDict(len(cd.configSettings))
	for name, def := range cd.configSettings {
		err := ret.SetKey(starlark.String(name), def.starlark())
		if err != nil {
			return starlark.None, w.String(), err
		}
	}

	return ret, w.String(), nil
}

// makes a new builtin with the given configValue constructor
// newConfigValue: a constructor for the `configValue` that we're making a function for
//              (it's the same logic for all types, except for the `configValue` that gets saved)
func configSettingDefinitionBuiltin(newConfigValue func() configValue) starkit.Function {
	return func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name string
		var isArgs bool
		var usage string
		err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
			"name",
			&name,
			"args?",
			&isArgs,
			"usage?",
			&usage,
		)
		if err != nil {
			return starlark.None, err
		}

		if name == "" {
			return starlark.None, errors.New("'name' is required")
		}

		err = starkit.SetState(thread, func(settings Settings) (Settings, error) {
			if _, ok := settings.configDef.configSettings[name]; ok {
				return settings, fmt.Errorf("%s defined multiple times", name)
			}

			if isArgs {
				if settings.configDef.positionalSettingName != "" {
					return settings, fmt.Errorf("both %s and %s are defined as positional args", name, settings.configDef.positionalSettingName)
				}

				settings.configDef.positionalSettingName = name
			}

			settings.configDef.configSettings[name] = configSetting{
				configValue: newConfigValue(),
				usage:       usage,
			}

			return settings, nil
		})
		if err != nil {
			return starlark.None, err
		}

		return starlark.None, nil
	}
}
