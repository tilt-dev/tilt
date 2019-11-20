package flags

import (
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
	"github.com/windmilleng/tilt/pkg/model"
)

type Settings struct {
	resources []string
}

type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return Settings{}
}

func (Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("flags.set_resources", setResources)
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

var _ starkit.StatefulExtension = Extension{}

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

func (s Settings) Resources(args []model.ManifestName) []model.ManifestName {
	if s.resources == nil {
		return args
	}

	var ret []model.ManifestName
	for _, r := range s.resources {
		ret = append(ret, model.ManifestName(r))
	}

	return ret
}
