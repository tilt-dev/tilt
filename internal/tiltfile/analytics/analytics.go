package analytics

import (
	"github.com/tilt-dev/wmclient/pkg/analytics"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

type Settings struct {
	Opt                analytics.Opt
	CustomTagsToReport map[string]string
}

type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return Settings{
		Opt:                analytics.OptDefault,
		CustomTagsToReport: make(map[string]string),
	}
}

func (Extension) OnStart(env *starkit.Environment) error {
	err := env.AddBuiltin("analytics_settings", setAnalyticsSettings)
	if err != nil {
		return err
	}

	// This is an experimental feature to allow Tiltfiles to specify custom data to report to analytics
	// to allow teams to get more visibility into, e.g., who's using Tilt or what k8s distributions are
	// their members using. It is not intended for use without coordinating with the Tilt team.
	err = env.AddBuiltin("experimental_report_custom_tags", reportCustomTags)
	if err != nil {
		return err
	}

	return nil
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

func reportCustomTags(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var tags value.StringStringMap
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "tags", &tags); err != nil {
		return nil, err
	}

	err := starkit.SetState(thread, func(settings Settings) Settings {
		for k, v := range tags {
			settings.CustomTagsToReport[k] = v
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
