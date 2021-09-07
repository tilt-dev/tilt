package metrics

import (
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type Plugin struct{}

func NewPlugin() Plugin {
	return Plugin{}
}

func (Plugin) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("experimental_metrics_settings", setMetricsSettings)
}

func setMetricsSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	ctx, err := starkit.ContextFromThread(thread)
	if err != nil {
		return nil, err
	}

	logger.Get(ctx).Warnf("experimental_metrics_settings() is deprecated")
	return starlark.None, nil
}

var _ starkit.Plugin = Plugin{}
