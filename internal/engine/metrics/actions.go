package metrics

import (
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type MetricsModeAction struct {
	Serving   store.MetricsServing
	Settings  model.MetricsSettings
	Manifests []model.Manifest
}

func (MetricsModeAction) Action() {}
