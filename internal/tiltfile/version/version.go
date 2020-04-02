package version

import (
	"fmt"

	"github.com/mcuadros/go-version"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/model"
)

type Extension struct {
	tiltVersion string
}

func NewExtension(tiltVersion string) Extension {
	return Extension{
		tiltVersion: tiltVersion,
	}
}

func (e Extension) NewState() interface{} {
	return model.VersionSettings{
		CheckUpdates: true,
	}
}

func (e Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("version_settings", e.setVersionSettings)
}

func (e Extension) setVersionSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// it's awkward to tell from UnpackArgs whether an optional value was set, so let's just make sure it has
	// the right initial value instead
	m, err := starkit.ModelFromThread(thread)
	if err != nil {
		return nil, errors.Wrap(err, "internal error reading version settings from starkit model")
	}
	state, err := GetState(m)
	if err != nil {
		return nil, errors.Wrap(err, "internal error reading version settings from starkit model")
	}

	checkUpdates := state.CheckUpdates
	var constraint string
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"check_updates?", &checkUpdates,
		"constraint?", &constraint,
	); err != nil {
		return nil, err
	}

	err = starkit.SetState(thread, func(settings model.VersionSettings) model.VersionSettings {
		settings.CheckUpdates = checkUpdates
		return settings
	})

	if constraint != "" {
		cg := version.NewConstrainGroupFromString(constraint)
		if !cg.Match(e.tiltVersion) {
			return nil, fmt.Errorf("you are running Tilt version %s, which doesn't match the version constraint specified in your Tiltfile: '%s'", e.tiltVersion, constraint)
		}
	}

	return starlark.None, err
}

var _ starkit.StatefulExtension = Extension{}

func MustState(model starkit.Model) model.VersionSettings {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (model.VersionSettings, error) {
	var state model.VersionSettings
	err := m.Load(&state)
	return state, err
}
