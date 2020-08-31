package metrics

import (
	"context"
	"crypto/tls"

	"contrib.go.opencensus.io/exporter/ocagent"
	"go.opencensus.io/stats/view"
	"google.golang.org/grpc/credentials"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Controller struct {
	exporter *DeferredExporter
	metrics  model.MetricsSettings
}

func NewController(exporter *DeferredExporter) *Controller {
	return &Controller{
		exporter: exporter,
	}
}

func (c *Controller) newMetricsSettings(rStore store.RStore) model.MetricsSettings {
	state := rStore.RLockState()
	defer rStore.RUnlockState()
	return state.MetricsSettings
}

func (c *Controller) OnChange(ctx context.Context, rStore store.RStore) {
	newMetricsSettings := c.newMetricsSettings(rStore)
	oldMetricsSettings := c.metrics
	if newMetricsSettings == oldMetricsSettings {
		return
	}

	c.metrics = newMetricsSettings
	view.SetReportingPeriod(newMetricsSettings.ReportingPeriod)

	if oldMetricsSettings.Enabled && !newMetricsSettings.Enabled {
		// shutdown the old metrics
		err := c.exporter.SetRemote(nil)
		if err != nil {
			logger.Get(ctx).Debugf("Shutting down metrics: %v", err)
		}
	}

	if newMetricsSettings.Enabled {
		// Replace the existing exporter.
		options := []ocagent.ExporterOption{
			ocagent.WithAddress(newMetricsSettings.Address),
			ocagent.WithServiceName("tilt"),
		}
		if newMetricsSettings.Insecure {
			options = append(options, ocagent.WithInsecure())
		} else {
			// default TLS config
			options = append(options, ocagent.WithTLSCredentials(credentials.NewTLS(&tls.Config{})))
		}
		oce, err := ocagent.NewExporter(options...)
		if err != nil {
			logger.Get(ctx).Debugf("Creating metrics exporter: %v", err)
			return
		}

		err = c.exporter.SetRemote(oce)
		if err != nil {
			logger.Get(ctx).Debugf("Setting metrics exporter: %v", err)
		}

		// TODO(nick): We need a mechanism to synchronously send the existing
		// aggregates to the remote.

	}
}
