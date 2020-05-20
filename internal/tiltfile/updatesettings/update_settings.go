package updatesettings

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

// Implements functions for dealing with update settings.
type Extension struct{}

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
	var maxParallelUpdates, k8sUpsertTimeoutSecs starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"max_parallel_updates?", &maxParallelUpdates,
		"k8s_upsert_timeout_secs?", &k8sUpsertTimeoutSecs); err != nil {
		return nil, err
	}

	mpu, mpuPassed, err := valueToInt(maxParallelUpdates)
	if err != nil {
		return nil, errors.Wrap(err, "update_settings: for parameter \"max_parallel_updates\"")
	}
	if mpuPassed && mpu < 1 {
		return nil, fmt.Errorf("max number of parallel updates must be >= 1(got: %d)",
			maxParallelUpdates)
	}

	kuts, kutsPassed, err := valueToInt(k8sUpsertTimeoutSecs)
	if err != nil {
		return nil, errors.Wrap(err, "update_settings: for parameter \"k8s_upsert_timeout_secs\"")
	}
	if kutsPassed && kuts < 1 {
		return nil, fmt.Errorf("minimum k8s upsert timeout is 1s; got %ds",
			k8sUpsertTimeoutSecs)
	}

	err = starkit.SetState(thread, func(settings model.UpdateSettings) model.UpdateSettings {
		if mpuPassed {
			settings = settings.WithMaxParallelUpdates(mpu)
		}
		if kutsPassed {
			settings = settings.WithK8sUpsertTimeout(time.Duration(kuts) * time.Second)
		}
		return settings
	})

	return starlark.None, err
}

func valueToInt(v starlark.Value) (val int, wasPassed bool, err error) {
	switch x := v.(type) {
	case nil, starlark.NoneType:
		return 0, false, nil
	case starlark.Int:
		val, err := starlark.AsInt32(x)
		return val, true, err
	default:
		return 0, true, fmt.Errorf("got %T, want int", x)
	}
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
