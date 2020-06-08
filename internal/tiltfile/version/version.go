package version

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Extension struct {
	tiltVersion string
}

func NewExtension(tiltBuild model.TiltBuild) Extension {
	return Extension{
		tiltVersion: tiltBuild.Version,
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
	var constraint string

	err := starkit.SetState(thread, func(settings model.VersionSettings) (model.VersionSettings, error) {
		if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
			"check_updates?", &settings.CheckUpdates,
			"constraint?", &constraint,
		); err != nil {
			return settings, err
		}
		return settings, nil
	})

	if constraint != "" {
		ver, err := semver.Parse(e.tiltVersion)
		if err != nil {
			return nil, errors.Wrapf(err, "internal error parsing tilt version '%s'", e.tiltVersion)
		}
		rng, err := semver.ParseRange(constraint)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing version constraint")
		}
		if !rng(ver) {
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
