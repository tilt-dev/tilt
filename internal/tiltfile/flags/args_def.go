package flags

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/model"
)

type argValue interface {
	flag.Value
	json.Marshaler
	starlark() starlark.Value
	setFromArgs([]string)
	setFromInterface(interface{}) error
	IsSet() bool
}

type argDef struct {
	newValue func() argValue
	usage    string
}

type ArgsDef struct {
	positionalArgName string
	args              map[string]argDef
}

func (ad ArgsDef) parse(flagsState model.FlagsState, args []string) (v starlark.Value, mergedArgs bool, output string, err error) {
	var config map[string]argValue
	config, err = ad.readFromFile(flagsState.ConfigPath)
	if err != nil {
		return starlark.None, false, "", err
	}

	// if we have not yet merged the current set of args, merge them into the flags from the file
	// and write them back out
	if flagsState.LastArgsWrite.IsZero() {
		var flagsFromArgs map[string]argValue
		flagsFromArgs, output, err = ad.parseArgs(args)
		if err != nil {
			return nil, false, output, err
		}

		for k, v := range flagsFromArgs {
			if v.IsSet() {
				config[k] = v
			}
		}

		f, err := os.Create(flagsState.ConfigPath)
		if err != nil {
			return nil, false, output, errors.Wrapf(err, "error opening %s for writing", flagsState.ConfigPath)
		}
		defer func() {
			err2 := f.Close()
			if err2 != nil && err == nil {
				err = errors.Wrapf(err2, "error closing %s", flagsState.ConfigPath)
			}
		}()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		err = json.NewEncoder(f).Encode(config)
		if err != nil {
			return nil, false, output, errors.Wrapf(err, "error serializing config to %s", flagsState.ConfigPath)
		}

		mergedArgs = true
	}

	ret := starlark.NewDict(len(ad.args))
	for k, v := range config {
		err := ret.SetKey(starlark.String(k), v.starlark())
		if err != nil {
			return nil, false, output, err
		}
	}

	return ret, mergedArgs, output, nil
}

func (ad ArgsDef) parseArgs(args []string) (ret map[string]argValue, output string, err error) {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	w := &bytes.Buffer{}
	fs.SetOutput(w)

	ret = make(map[string]argValue)
	for name, def := range ad.args {
		ret[name] = def.newValue()
		if name == ad.positionalArgName {
			continue
		}
		fs.Var(ret[name], name, def.usage)
	}

	err = fs.Parse(args)
	if err != nil {
		return nil, w.String(), err
	}

	if len(fs.Args()) > 0 {
		if ad.positionalArgName == "" {
			return nil, w.String(), errors.New("positional args were specified, but none were expected (no arg defined with args=True)")
		} else {
			ret[ad.positionalArgName].setFromArgs(fs.Args())
		}
	}

	return ret, w.String(), nil
}

func (ad ArgsDef) readFromFile(tiltConfigPath string) (ret map[string]argValue, err error) {
	ret = make(map[string]argValue)
	r, err := os.Open(tiltConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ret, nil
		}
		return nil, errors.Wrapf(err, "error opening %s", tiltConfigPath)
	}
	defer func() {
		err2 := r.Close()
		if err2 != nil && err == nil {
			err = errors.Wrapf(err2, "error closing %s", tiltConfigPath)
		}
	}()

	m := make(map[string]interface{})
	err = jsoniter.NewDecoder(r).Decode(&m)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing json from %s", tiltConfigPath)
	}

	for k, v := range m {
		def, ok := ad.args[k]
		if !ok {
			return nil, fmt.Errorf("%s specified unknown flag name '%s'", tiltConfigPath, k)
		}
		ret[k] = def.newValue()
		err = ret[k].setFromInterface(v)
		if err != nil {
			return nil, errors.Wrapf(err, "%s specified invalid value for flag %s", tiltConfigPath, k)
		}
	}
	return ret, nil
}

// makes a new builtin with the given argValue constructor
// newArgValue: a constructor for the `argValue` that we're making a function for
//              (it's the same logic for all types, except for the `argValue` that gets saved)
func argDefinitionBuiltin(newArgValue func() argValue) starkit.Function {
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
			if _, ok := settings.argDef.args[name]; ok {
				return settings, fmt.Errorf("%s defined multiple times", name)
			}

			if isArgs {
				if settings.argDef.positionalArgName != "" {
					return settings, fmt.Errorf("both %s and %s are defined as positional args", name, settings.argDef.positionalArgName)
				}

				settings.argDef.positionalArgName = name
			}

			settings.argDef.args[name] = argDef{
				newValue: newArgValue,
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
