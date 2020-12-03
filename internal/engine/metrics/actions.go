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

// Broadcasts information about the Grafana dashboard.
// In the future, this may come from cloud.tilt.dev.
type MetricsDashboardAction struct {
	GrafanaHost string
}

func (MetricsDashboardAction) Action() {}
