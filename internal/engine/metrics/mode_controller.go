package metrics

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/user"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const grafanaManifestName = model.ManifestName("(tilt-grafana)")
const collectorManifestName = model.ManifestName("(tilt-collector)")
const promManifestName = model.ManifestName("(tilt-prometheus)")
const collectorHostPort = 10351
const grafanaHostPort = 10352

func IsLocalMetricsStack(name model.ManifestName) bool {
	return name == grafanaManifestName ||
		name == collectorManifestName ||
		name == promManifestName
}

type ModeController struct {
	host        model.WebHost
	userPrefs   user.PrefsInterface
	initialized bool
}

func NewModeController(host model.WebHost, userPrefs user.PrefsInterface) *ModeController {
	return &ModeController{host: host, userPrefs: userPrefs}
}

func (c *ModeController) currentMode(rStore store.RStore) model.MetricsMode {
	state := rStore.RLockState()
	defer rStore.RUnlockState()
	return state.MetricsServing.Mode
}

// TODO(nick): Eventually we will need to reconcile multiple signals for
// enabling metrics:
//
// 1) A team setting on cloud.tilt.dev
// 2) A personal setting under ~/.tilt-dev
// 3) An environment variable
// 4) A Tiltfile setting
//
// As well as different signals on where to send those metrics
// (to a local collector vs a remote collector).
func (c *ModeController) desiredMetricsMode() model.MetricsMode {
	// Env variable taks precedence over everything else.
	envMode := model.MetricsMode(os.Getenv("TILT_METRICS"))
	if envMode == model.MetricsLocal || envMode == model.MetricsDisabled {
		return envMode
	}

	// The personal setting is next.
	userPrefs, _ := c.userPrefs.Get()
	userMode := model.MetricsMode(userPrefs.MetricsMode)
	if userMode == model.MetricsLocal || userMode == model.MetricsDisabled {
		return userMode
	}

	return model.MetricsDefault
}

func (c *ModeController) SetUserMode(ctx context.Context, rStore store.RStore, newMode model.MetricsMode) {
	currentMode := c.currentMode(rStore)
	if currentMode == newMode {
		return
	}

	err := user.UpdateMetricsMode(c.userPrefs, newMode)
	if err != nil {
		logger.Get(ctx).Debugf("writing metrics mode: %v", err)
		return
	}

	if newMode == model.MetricsLocal {
		c.setMetricsLocal(ctx, rStore)
		return
	}
	c.setMetricsNone(ctx, rStore)
}

func (c *ModeController) OnChange(ctx context.Context, rStore store.RStore) {
	if c.initialized {
		return
	}
	c.initialized = true

	mode := c.desiredMetricsMode()
	if mode != model.MetricsLocal {
		return
	}

	c.setMetricsLocal(ctx, rStore)
}

func (c *ModeController) setMetricsNone(ctx context.Context, rStore store.RStore) {
	rStore.Dispatch(MetricsModeAction{
		Serving: store.MetricsServing{
			Mode: model.MetricsDefault,
		},
	})
}

func (c *ModeController) setMetricsLocal(ctx context.Context, rStore store.RStore) {
	stack, err := c.localMetricsStack()
	if err != nil {
		logger.Get(ctx).Warnf("metrics mode: %v", err)
		return
	}

	rStore.Dispatch(MetricsModeAction{
		Serving: store.MetricsServing{
			Mode:        model.MetricsLocal,
			GrafanaHost: fmt.Sprintf("%s:%d", c.host, grafanaHostPort),
		},
		Settings: model.MetricsSettings{
			Enabled:         true,
			Address:         fmt.Sprintf("%s:%d", c.host, collectorHostPort),
			Insecure:        true,
			ReportingPeriod: 5 * time.Second,
			AllowAnonymous:  true,
		},
		Manifests: stack,
	})
}

// The metrics stack consists of 3 servers: a collector (for ingestion),
// prometheus (for querying/indexing timeseries data),
// and grafana (for displaying the timeseries).
//
// In an ideal world, this would be only one manifest (or a resource group),
// but our port-forwarding system only supports 1 pod per manifest.
func (c *ModeController) localMetricsStack() ([]model.Manifest, error) {
	collector, err := c.localMetricsManifest(collectorManifestName, []string{
		collector,
		collectorConfig,
	}, []model.PortForward{{ContainerPort: 55678, LocalPort: collectorHostPort, Host: string(c.host)}}, nil)
	if err != nil {
		return nil, errors.Wrap(err, "init metrics collector")
	}

	prometheus, err := c.localMetricsManifest(promManifestName, []string{
		prometheus,
		prometheusConfig,
	}, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "init metrics prometheus")
	}

	// Hash the grafana config so that the grafana server reloads if it changes.
	configHash := md5.Sum([]byte(grafanaConfig + grafanaDashboardConfig))
	grafanaLabels := []model.LabelPair{
		model.LabelPair{Key: "tilt.dev/config-hash", Value: fmt.Sprintf("%x", configHash)},
	}

	grafana, err := c.localMetricsManifest(grafanaManifestName, []string{
		grafana,
		grafanaConfig,
		grafanaDashboardConfig,
	}, []model.PortForward{{ContainerPort: 3000, LocalPort: grafanaHostPort, Host: string(c.host)}}, grafanaLabels)
	if err != nil {
		return nil, errors.Wrap(err, "init metrics grafana")
	}
	return []model.Manifest{collector, prometheus, grafana}, nil
}

func (c *ModeController) localMetricsManifest(name model.ManifestName, yaml []string, ports []model.PortForward, labels []model.LabelPair) (model.Manifest, error) {
	entities := []k8s.K8sEntity{}
	for _, c := range yaml {
		newEs, err := k8s.ParseYAML(strings.NewReader(c))
		if err != nil {
			return model.Manifest{}, fmt.Errorf("init local metrics: %v", err)
		}
		entities = append(entities, newEs...)
	}

	if len(labels) > 0 {
		for i, e := range entities {
			e, err := k8s.InjectLabels(e, labels)
			if err != nil {
				return model.Manifest{}, fmt.Errorf("init local metrics: %v", err)
			}
			entities[i] = e
		}
	}

	kTarget, err := k8s.NewTarget(model.TargetName(name), entities, ports,
		nil, nil, nil, model.PodReadinessIgnore, nil, nil)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("init local metrics: %v", err)
	}
	return model.Manifest{
		Name:   name,
		Source: model.ManifestSourceMetrics,
	}.WithDeployTarget(kTarget), nil
}
