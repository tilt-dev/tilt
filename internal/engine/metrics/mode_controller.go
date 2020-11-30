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
	initialized bool
}

func NewModeController(host model.WebHost) *ModeController {
	return &ModeController{host: host}
}

func (c *ModeController) currentMode(rStore store.RStore) store.MetricsServing {
	state := rStore.RLockState()
	defer rStore.RUnlockState()
	return state.MetricsServing
}

func (c *ModeController) OnChange(ctx context.Context, rStore store.RStore) {
	if c.initialized {
		return
	}
	c.initialized = true

	// NOTE(nick): This is a hack until we have a real flow for
	// letting the user set the metrics mode from the UI, or letting
	// their team lead set it from a dashboard.
	localMetricsEnv := os.Getenv("TILT_LOCAL_METRICS")
	if localMetricsEnv != "1" {
		return
	}

	stack, err := c.localMetricsStack()
	if err != nil {
		logger.Get(ctx).Warnf("metrics mode: %v", err)
		return
	}

	rStore.Dispatch(MetricsModeAction{
		Serving: store.MetricsServing{
			Mode:        store.MetricsLocal,
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
