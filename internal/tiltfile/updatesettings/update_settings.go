package updatesettings

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
)

// Implements functions for dealing with update settings.
type Plugin struct{}

func NewPlugin() Plugin {
	return Plugin{}
}

func (e Plugin) NewState() interface{} {
	return model.DefaultUpdateSettings()
}

func (e Plugin) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("update_settings", e.updateSettings)
}

func (e *Plugin) updateSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var maxParallelUpdates, k8sUpsertTimeoutSecs starlark.Value
	var unusedImageWarnings value.StringOrStringList
	var k8sServerSideApply string
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"max_parallel_updates?", &maxParallelUpdates,
		"k8s_upsert_timeout_secs?", &k8sUpsertTimeoutSecs,
		"suppress_unused_image_warnings?", &unusedImageWarnings,
		"k8s_server_side_apply?", &k8sServerSideApply); err != nil {
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

	if k8sServerSideApply != "" && k8sServerSideApply != "true" && k8sServerSideApply != "false" && k8sServerSideApply != "auto" {
		return nil, fmt.Errorf("update_settings: k8s_server_side_apply must be \"true\", \"false\", or \"auto\"; got %q", k8sServerSideApply)
	}

	err = starkit.SetState(thread, func(settings model.UpdateSettings) model.UpdateSettings {
		if mpuPassed {
			settings = settings.WithMaxParallelUpdates(mpu)
		}
		if kutsPassed {
			settings = settings.WithK8sUpsertTimeout(time.Duration(kuts) * time.Second)
		}
		settings.SuppressUnusedImageWarnings = append(settings.SuppressUnusedImageWarnings, unusedImageWarnings.Values...)
		if k8sServerSideApply != "" {
			settings = settings.WithK8sServerSideApply(k8sServerSideApply)
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

var _ starkit.StatefulPlugin = Plugin{}

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
