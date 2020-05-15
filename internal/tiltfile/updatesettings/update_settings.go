package updatesettings

import (
	"fmt"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

// Implements functions for dealing with update settings.
type Extension struct {
	callPosition syntax.Position
}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return model.DefaultUpdateSettings()
}

func (e Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("update_settings", e.updateSettings)
}

func (e *Extension) updateSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if e.callPosition.IsValid() {
		return starlark.None, fmt.Errorf(
			"'update_settings' can only be called once. It was already called at %s", e.callPosition.String())
	}
	e.callPosition = thread.CallFrame(1).Pos

	maxParallelUpdates := model.DefaultMaxParallelUpdates
	k8sUpsertTimeoutSecs := int(model.DefaultK8sUpsertTimeout / time.Second)

	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"max_parallel_updates?", &maxParallelUpdates,
		"k8s_upsert_timeout_secs?", &k8sUpsertTimeoutSecs); err != nil {
		return nil, err
	}

	if maxParallelUpdates < 1 {
		return nil, fmt.Errorf("max number of parallel updates must be >= 1(got: %d)",
			maxParallelUpdates)
	}
	if k8sUpsertTimeoutSecs < 1 {
		return nil, fmt.Errorf("minimum k8s upsert timeout is 1s; got %ds",
			k8sUpsertTimeoutSecs)
	}

	err := starkit.SetState(thread, func(settings model.UpdateSettings) model.UpdateSettings {
		settings = settings.WithMaxParallelUpdates(maxParallelUpdates)
		settings = settings.WithK8sUpsertTimeout(time.Duration(k8sUpsertTimeoutSecs) * time.Second)
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
