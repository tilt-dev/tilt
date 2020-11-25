package store

import "github.com/tilt-dev/tilt/pkg/model"

type MetricsServing struct {
	Mode        model.MetricsMode
	GrafanaHost string
}
