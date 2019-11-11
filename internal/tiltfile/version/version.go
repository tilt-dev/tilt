package version

import (
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/model"
)

type Extension struct{}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return model.VersionSettings{
		CheckUpdates: true,
	}
}

func (Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("version_settings", setVersionSettings)
}

func setVersionSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var checkUpdates bool
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"check_updates", &checkUpdates); err != nil {
		return nil, err
	}

	err := starkit.SetState(thread, func(settings model.VersionSettings) model.VersionSettings {
		if checkUpdates {
			settings.CheckUpdates = true
		} else {
			settings.CheckUpdates = false
		}
		return settings
	})

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
