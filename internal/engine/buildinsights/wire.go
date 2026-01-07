package buildinsights

import (
	"github.com/tilt-dev/tilt/internal/xdg"
	"github.com/tilt-dev/tilt/pkg/model"
)

// ProvideInsightsStore creates a new build insights store.
// This is used for dependency injection via Wire.
func ProvideInsightsStore(xdgBase xdg.Base) (model.BuildInsightsStore, error) {
	return NewFileStore(xdgBase)
}

// ProvideCollector creates a new build insights collector.
// This is used for dependency injection via Wire.
func ProvideCollector(store model.BuildInsightsStore) *Collector {
	return NewCollector(store)
}
