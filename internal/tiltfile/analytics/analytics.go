package analytics

import (
	"github.com/windmilleng/wmclient/pkg/analytics"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type Settings struct {
	Opt analytics.Opt
}

type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return Settings{
		Opt: analytics.OptDefault,
	}
}

func (Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("analytics_settings", setAnalyticsSettings)
}

func setAnalyticsSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var enable bool
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"enable", &enable); err != nil {
		return nil, err
	}

	err := starkit.SetState(thread, func(settings Settings) Settings {
		if enable {
			settings.Opt = analytics.OptIn
		} else {
			settings.Opt = analytics.OptOut
		}
		return settings
	})

	return starlark.None, err
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
