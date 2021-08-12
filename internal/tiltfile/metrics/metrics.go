package metrics

import (
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Plugin struct{}

func NewPlugin() Plugin {
	return Plugin{}
}

func (e Plugin) NewState() interface{} {
	return model.DefaultMetricsSettings()
}

func (Plugin) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("experimental_metrics_settings", setMetricsSettings)
}

func setMetricsSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	err := starkit.SetState(thread, func(settings model.MetricsSettings) (model.MetricsSettings, error) {
		var reportingPeriod value.Duration
		err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
			"enabled?", &settings.Enabled,
			"address?", &settings.Address,
			"insecure?", &settings.Insecure,
			"reporting_period?", &reportingPeriod)
		if err != nil {
			return model.MetricsSettings{}, err
		}

		if !reportingPeriod.IsZero() {
			settings.ReportingPeriod = reportingPeriod.AsDuration()
		}
		return settings, nil
	})

	return starlark.None, err
}

var _ starkit.StatefulPlugin = Plugin{}

func MustState(model starkit.Model) model.MetricsSettings {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (model.MetricsSettings, error) {
	var state model.MetricsSettings
	err := m.Load(&state)
	return state, err
}
