package updatesettings

import (
	"fmt"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

// Implements functions for dealing with Docker Prune settings.
type Extension struct {
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return model.DefaultUpdateSettings()
}

func (e Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("max_parallel_updates", e.maxParallelUpdates)
}

func (e Extension) maxParallelUpdates(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var n int
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"n?", &n); err != nil {
		return nil, err
	}

	if n < 1 {
		return nil, fmt.Errorf("max number of parallel updates must be >= 1(got: %d)", n)
	}

	err := starkit.SetState(thread, func(settings model.UpdateSettings) model.UpdateSettings {
		settings.MaxBuildSlots = n
		return settings
	})

	return starlark.None, err
}

var _ starkit.StatefulExtension = Extension{}

func MustState(model starkit.Model) model.UpdateSettings {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (model.UpdateSettings, error) {
	var state model.UpdateSettings
	err := m.Load(&state)
	return state, err
}
