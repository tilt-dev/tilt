package config

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type configValue interface {
	flag.Value
	starlark() starlark.Value
	setFromInterface(interface{}) error
	IsSet() bool
}

type configMap map[string]configValue

type configSetting struct {
	newValue func() configValue
	usage    string
}

type ConfigDef struct {
	positionalSettingName string
	configSettings        map[string]configSetting
}

func (cm configMap) toStarlark() (starlark.Mapping, error) {
	ret := starlark.NewDict(len(cm))
	for k, v := range cm {
		err := ret.SetKey(starlark.String(k), v.starlark())
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// merges settings from config and settings from args, with settings from args trumping
func mergeConfigMaps(settingsFromConfig, settingsFromArgs configMap) configMap {
	ret := make(configMap)
	for k, v := range settingsFromConfig {
		ret[k] = v
	}

	for k, v := range settingsFromArgs {
		if v.IsSet() {
			ret[k] = v
		}
	}

	return ret
}

// parse any args and merge them into the config
func (cd ConfigDef) incorporateArgs(config configMap, args []string) (ret configMap, output string, err error) {
	var settingsFromArgs configMap
	settingsFromArgs, output, err = cd.parseArgs(args)
	if err != nil {
		return nil, output, err
	}

	config = mergeConfigMaps(config, settingsFromArgs)

	return config, output, nil
}

func (cd ConfigDef) parse(configPath string, args []string) (v starlark.Value, output string, err error) {
	config, err := cd.readFromFile(configPath)
	if err != nil {
		return starlark.None, "", err
	}

	config, output, err = cd.incorporateArgs(config, args)
	if err != nil {
		return starlark.None, output, err
	}

	ret, err := config.toStarlark()
	if err != nil {
		return nil, output, err
	}

	return ret, output, nil
}

// parse command-line args
func (cd ConfigDef) parseArgs(args []string) (ret configMap, output string, err error) {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	w := &bytes.Buffer{}
	fs.SetOutput(w)

	ret = make(configMap)
	for name, def := range cd.configSettings {
		ret[name] = def.newValue()
		if name == cd.positionalSettingName {
			continue
		}
		fs.Var(ret[name], name, def.usage)
	}

	err = fs.Parse(args)
	if err != nil {
		return nil, w.String(), err
	}

	if len(fs.Args()) > 0 {
		if cd.positionalSettingName == "" {
			return nil, w.String(), errors.New("positional args were specified, but none were expected (no setting defined with args=True)")
		} else {
			for _, arg := range fs.Args() {
				err := ret[cd.positionalSettingName].Set(arg)
				if err != nil {
					return nil, w.String(), errors.Wrapf(err, "error setting positional arg %s", cd.positionalSettingName)
				}
			}
		}
	}

	return ret, w.String(), nil
}

// parse settings from the config file
func (cd ConfigDef) readFromFile(tiltConfigPath string) (ret configMap, err error) {
	ret = make(configMap)
	r, err := os.Open(tiltConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ret, nil
		}
		return nil, errors.Wrapf(err, "error opening %s", tiltConfigPath)
	}
	defer func() {
		_ = r.Close()
	}()

	m := make(map[string]interface{})
	err = jsoniter.NewDecoder(r).Decode(&m)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing json from %s", tiltConfigPath)
	}

	for k, v := range m {
		def, ok := cd.configSettings[k]
		if !ok {
			return nil, fmt.Errorf("%s specified unknown setting name '%s'", tiltConfigPath, k)
		}
		ret[k] = def.newValue()
		err = ret[k].setFromInterface(v)
		if err != nil {
			return nil, errors.Wrapf(err, "%s specified invalid value for setting %s", tiltConfigPath, k)
		}
	}
	return ret, nil
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
			if settings.configParseCalled {
				return settings, fmt.Errorf("%s cannot be called after config.parse is called", fn.Name())
			}

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
				newValue: newConfigValue,
				usage:    usage,
			}

			return settings, nil
		})
		if err != nil {
			return starlark.None, err
		}

		return starlark.None, nil
	}
}
