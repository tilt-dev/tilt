package flags

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
	"github.com/windmilleng/tilt/pkg/model"
)

type Settings struct {
	resources []string
	argDef    ArgsDef

	flagsParsed bool
}

type Extension struct {
	cmdLineArgs []string
}

func NewExtension() *Extension {
	return &Extension{}
}

func (e *Extension) NewState() interface{} {
	return Settings{
		argDef: ArgsDef{
			args: make(map[string]argDef),
		},
	}
}

var _ starkit.StatefulExtension = &Extension{}

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
	err := env.AddBuiltin("flags.set_resources", setResources)
	if err != nil {
		return errors.Wrap(err, "flags.set_resources")
	}
	err = env.AddBuiltin("flags.define_string_list", defineStringList)
	if err != nil {
		return errors.Wrap(err, "flags.define_string_list")
	}
	err = env.AddBuiltin("flags.parse", e.parse)
	if err != nil {
		return errors.Wrap(err, "flags.parse")
	}

	e.cmdLineArgs = env.CmdLineArgs()

	return nil
}

func setResources(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var slResources starlark.Sequence
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"resources",
		&slResources,
	)
	if err != nil {
		return starlark.None, err
	}

	resources, err := value.SequenceToStringSlice(slResources)
	if err != nil {
		return starlark.None, errors.Wrap(err, "resources must be a list of string")
	}

	err = starkit.SetState(thread, func(settings Settings) Settings {
		settings.resources = resources
		return settings
	})
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
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

	st, err := starkit.GetState(thread, Settings{})
	if err != nil {
		return starlark.None, err
	}

	return st.(Settings).argDef.parse(e.cmdLineArgs, starkit.NewThreadWriter(thread))
}

func defineStringList(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
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
			typ:   argTypeStringList,
			usage: usage,
		}

		return settings, nil
	})
	if err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

type argType int

const (
	argTypeStringList argType = iota
)

type argDef struct {
	typ   argType
	usage string
}

type ArgsDef struct {
	positionalArgName string
	args              map[string]argDef
}

// Strings is a `flag.Value` for `string` arguments. (from https://github.com/sgreben/flagvar/blob/master/string.go)
type Strings struct {
	Values []string
}

// Set is flag.Value.Set
func (fv *Strings) Set(v string) error {
	fv.Values = append(fv.Values, v)
	return nil
}

func (fv *Strings) String() string {
	return strings.Join(fv.Values, ",")
}

func (ad ArgsDef) parse(args []string, w io.Writer) (starlark.Value, error) {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.SetOutput(w)
	stringListValues := make(map[string]*Strings)
	for name, def := range ad.args {
		if name == ad.positionalArgName {
			continue
		}
		switch def.typ {
		case argTypeStringList:
			stringListValues[name] = &Strings{}
			fs.Var(stringListValues[name], name, def.usage)
		}
	}

	err := fs.Parse(args)
	if err != nil {
		return starlark.None, err
	}

	if len(fs.Args()) > 0 && ad.positionalArgName == "" {
		return starlark.None, errors.New("positional args were specified, but none were expected (no arg defined with args=True)")
	}

	ret := starlark.NewDict(len(ad.args))
	for name, def := range ad.args {
		switch def.typ {
		case argTypeStringList:
			var v []string
			if name == ad.positionalArgName {
				v = fs.Args()
			} else {
				v = stringListValues[name].Values
			}
			err := ret.SetKey(starlark.String(name), value.StringSliceToList(v))
			if err != nil {
				return starlark.None, err
			}
		}
	}

	return ret, nil
}

// for the given args and list of full manifests, figure out which manifests the user actually selected
func (s Settings) Resources(args []string, allManifests []model.ManifestName) []model.ManifestName {
	// if the user called set_resources, that trumps everything
	if s.resources != nil {
		var ret []model.ManifestName
		for _, r := range s.resources {
			ret = append(ret, model.ManifestName(r))
		}
		return ret
	}

	// if the user has not called flags.parse and has specified args, use those to select which resources
	if args != nil && !s.flagsParsed {
		var ret []model.ManifestName
		for _, arg := range args {
			ret = append(ret, model.ManifestName(arg))
		}
		return ret
	}

	// otherwise, they get everything
	return allManifests
}
