package dockerprune

import (
	"fmt"
	"time"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

// Implements functions for dealing with the Kubernetes context.
// Exposes an API for other plugins to get and validate the allowed k8s context.
type Extension struct {
	settings model.DockerPruneSettings
}

func NewExtension() *Extension {
	return &Extension{
		settings: model.DockerPruneSettings{
			Enabled: true,
		},
	}
}

func (e *Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("docker_prune_settings", e.dockerPruneSettings)
}

func (e *Extension) dockerPruneSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var disable bool
	var intervalHrs, numBuilds, maxAgeMins int
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"disable?", &disable,
		"max_age_mins?", &maxAgeMins,
		"num_builds?", &numBuilds,
		"interval_hrs?", &intervalHrs); err != nil {
		return nil, err
	}

	if disable && (intervalHrs != 0 || numBuilds != 0 || maxAgeMins != 0) {
		return nil, fmt.Errorf("can't disable Docker Prune (`disabled=True`) and pass additional settings")
	}

	if numBuilds != 0 && intervalHrs != 0 {
		return nil, fmt.Errorf("can't specify both 'prune every X builds' and 'prune every Y hours'; please pass " +
			"only one of `num_builds` and `interval_hrs`")
	}

	e.settings.Enabled = !disable
	e.settings.MaxAge = time.Duration(maxAgeMins) * time.Minute
	e.settings.NumBuilds = numBuilds
	e.settings.Interval = time.Duration(intervalHrs) * time.Hour

	return starlark.None, nil
}

func (e *Extension) Settings() model.DockerPruneSettings {
	return e.settings
}
