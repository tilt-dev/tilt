package flags

import (
	"bytes"
	"errors"
	"flag"
	"fmt"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type argValue interface {
	flag() flag.Value
	starlark() starlark.Value
	setFromArgs([]string)
}

type argDef struct {
	argValue
	usage string
}

type ArgsDef struct {
	positionalArgName string
	args              map[string]argDef
}

func (ad ArgsDef) parse(args []string) (v starlark.Value, output string, err error) {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	w := &bytes.Buffer{}
	fs.SetOutput(w)

	for name, def := range ad.args {
		if name == ad.positionalArgName {
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
		if ad.positionalArgName == "" {
			return starlark.None, w.String(), errors.New("positional args were specified, but none were expected (no arg defined with args=True)")
		} else {
			ad.args[ad.positionalArgName].setFromArgs(fs.Args())
		}
	}

	ret := starlark.NewDict(len(ad.args))
	for name, def := range ad.args {
		err := ret.SetKey(starlark.String(name), def.starlark())
		if err != nil {
			return starlark.None, w.String(), err
		}
	}

	return ret, w.String(), nil
}

// makes a new builtin with the given argValue constructor
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
				argValue: newArgValue(),
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
