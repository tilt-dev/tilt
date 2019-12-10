package config

import (
	"bytes"
	"errors"
	"flag"
	"fmt"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type flagValue interface {
	flag() flag.Value
	starlark() starlark.Value
	setFromArgs([]string)
}

type flagDef struct {
	flagValue
	usage string
}

type FlagsDef struct {
	positionalFlagName string
	flagDefs           map[string]flagDef
}

func (ad FlagsDef) parse(args []string) (v starlark.Value, output string, err error) {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	w := &bytes.Buffer{}
	fs.SetOutput(w)

	for name, def := range ad.flagDefs {
		if name == ad.positionalFlagName {
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
		if ad.positionalFlagName == "" {
			return starlark.None, w.String(), errors.New("positional args were specified, but none were expected (no flag defined with args=True)")
		} else {
			ad.flagDefs[ad.positionalFlagName].setFromArgs(fs.Args())
		}
	}

	ret := starlark.NewDict(len(ad.flagDefs))
	for name, def := range ad.flagDefs {
		err := ret.SetKey(starlark.String(name), def.starlark())
		if err != nil {
			return starlark.None, w.String(), err
		}
	}

	return ret, w.String(), nil
}

// makes a new builtin with the given flagValue constructor
// newFlagValue: a constructor for the `flagValue` that we're making a function for
//              (it's the same logic for all types, except for the `flagValue` that gets saved)
func flagDefinitionBuiltin(newFlagValue func() flagValue) starkit.Function {
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
			if _, ok := settings.flagsDef.flagDefs[name]; ok {
				return settings, fmt.Errorf("%s defined multiple times", name)
			}

			if isArgs {
				if settings.flagsDef.positionalFlagName != "" {
					return settings, fmt.Errorf("both %s and %s are defined as positional args", name, settings.flagsDef.positionalFlagName)
				}

				settings.flagsDef.positionalFlagName = name
			}

			settings.flagsDef.flagDefs[name] = flagDef{
				flagValue: newFlagValue(),
				usage:     usage,
			}

			return settings, nil
		})
		if err != nil {
			return starlark.None, err
		}

		return starlark.None, nil
	}
}
