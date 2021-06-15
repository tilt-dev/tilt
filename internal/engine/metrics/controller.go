package metrics

import (
	"context"
	"crypto/tls"
	"runtime"

	"contrib.go.opencensus.io/exporter/ocagent"
	"go.opencensus.io/resource"
	"go.opencensus.io/stats/view"
	"google.golang.org/grpc/credentials"

	"github.com/tilt-dev/tilt/internal/git"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type MetricsState struct {
	settings model.MetricsSettings
	token    token.Token
	username string
	teamID   string
}

func (s MetricsState) Enabled() bool {
	if !s.settings.Enabled {
		return false
	}
	if s.settings.AllowAnonymous {
		return true
	}
	return s.username != "" && s.token != ""
}

type Controller struct {
	exporter  *DeferredExporter
	metrics   MetricsState
	tiltBuild model.TiltBuild
	gitRemote git.GitRemote
}

var _ store.Subscriber = &Controller{}

func NewController(exporter *DeferredExporter, tiltBuild model.TiltBuild, gitRemote git.GitRemote) *Controller {
	return &Controller{
		exporter:  exporter,
		tiltBuild: tiltBuild,
		gitRemote: gitRemote,
	}
}

func (c *Controller) newMetricsState(rStore store.RStore) MetricsState {
	state := rStore.RLockState()
	defer rStore.RUnlockState()
	return MetricsState{
		settings: state.MetricsSettings,
		token:    state.Token,
		username: state.CloudStatus.Username,
		teamID:   state.TeamID,
	}
}

func (c *Controller) OnChange(ctx context.Context, rStore store.RStore, _ store.ChangeSummary) error {
	newMetricsState := c.newMetricsState(rStore)
	oldMetricsState := c.metrics
	if newMetricsState == oldMetricsState {
		return nil
	}

	c.metrics = newMetricsState
	view.SetReportingPeriod(newMetricsState.settings.ReportingPeriod)

	if oldMetricsState.Enabled() && !newMetricsState.Enabled() {
		// shutdown the old metrics
		err := c.exporter.SetRemote(nil)
		if err != nil {
			logger.Get(ctx).Debugf("Shutting down metrics: %v", err)
		}
	}

	if newMetricsState.Enabled() {
		// Replace the existing exporter.
		options := []ocagent.ExporterOption{
			ocagent.WithAddress(newMetricsState.settings.Address),
			ocagent.WithServiceName("tilt"),
			ocagent.WithResourceDetector(c.makeResourceDetector(newMetricsState)),
		}
		if newMetricsState.settings.Insecure {
			options = append(options, ocagent.WithInsecure())
		} else {
			// default TLS config
			options = append(options, ocagent.WithTLSCredentials(credentials.NewTLS(&tls.Config{})))
		}
		oce, err := ocagent.NewExporter(options...)
		if err != nil {
			logger.Get(ctx).Debugf("Creating metrics exporter: %v", err)
			return nil
		}

		err = c.exporter.SetRemote(oce)
		if err != nil {
			logger.Get(ctx).Debugf("Setting metrics exporter: %v", err)
		}
	}

	if newMetricsState.Enabled() && !oldMetricsState.Enabled() {
		// If we're exporting for the first time, flush now.
		c.exporter.Flush()
	}

	return nil
}

func (c *Controller) makeResourceDetector(state MetricsState) func(ctx context.Context) (*resource.Resource, error) {
	return func(ctx context.Context) (*resource.Resource, error) {
		return &resource.Resource{
			Type: "tilt.dev/tilt",
			Labels: map[string]string{
				"os":         runtime.GOOS,
				"version":    c.tiltBuild.AnalyticsVersion(),
				"git_origin": c.gitRemote.String(),
				"team":       state.teamID,

				// We'll inject the username server-side from the token.
				"token": state.token.String(),
			},
		}, nil
	}
}
