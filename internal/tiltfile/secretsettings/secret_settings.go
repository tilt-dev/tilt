package secretsettings

import (
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

// Implements functions for dealing with k8s secret settings.
type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return model.DefaultSecretSettings()
}

func (e Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("secret_settings", e.secretSettings)
}

func (e Extension) secretSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var disable bool
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"disable_scrub", &disable); err != nil {
		return nil, err
	}

	err := starkit.SetState(thread, func(settings model.SecretSettings) model.SecretSettings {
		settings.ScrubSecrets = !disable
		return settings
	})

	return starlark.None, err
}

var _ starkit.StatefulExtension = Extension{}

func MustState(model starkit.Model) model.SecretSettings {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (model.SecretSettings, error) {
	var state model.SecretSettings
	err := m.Load(&state)
	return state, err
}
