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

type ModeController struct {
	host        model.WebHost
	userPrefs   user.PrefsInterface
	initialized bool
}

var _ store.Subscriber = &ModeController{}

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

// When the grafana dashboard becomes ready, fire an action to tell
// the UI where to find it.
func (c *ModeController) checkReady(ctx context.Context, rStore store.RStore) {
	state := rStore.RLockState()
	defer rStore.RUnlockState()

	if state.MetricsServing.Mode != model.MetricsLocal {
		// No readiness check needed.
		return
	}

	if state.MetricsServing.GrafanaHost != "" {
		// Grafana URL already populated.
		return
	}

	ms, ok := state.ManifestState(grafanaManifestName)
	if !ok {
		return
	}

	status := ms.K8sRuntimeState().RuntimeStatus()
	if status != model.RuntimeStatusOK {
		return
	}

	rStore.Dispatch(MetricsDashboardAction{
		GrafanaHost: fmt.Sprintf("%s:%d", c.host, grafanaHostPort),
	})
}

func (c *ModeController) OnChange(ctx context.Context, rStore store.RStore, _ store.ChangeSummary) {
	c.initialize(ctx, rStore)
	c.checkReady(ctx, rStore)
}

func (c *ModeController) initialize(ctx context.Context, rStore store.RStore) {
	if c.initialized {
		return
	}

	mode := c.desiredMetricsMode()
	if mode != model.MetricsLocal {
		c.initialized = true
		return
	}

	if !c.supportsLocalMetricsStack(rStore) {
		// Check again later.
		return
	}

	c.setMetricsLocal(ctx, rStore)
	c.initialized = true
}

// Before deploying a local metrics stack, we want to check if:
// 1) The user is running Kubernetes
// 2) The user is trying to deploy to a valid Kubernetes cluster (i.e., we don't
//    want to accidentally deploy to a prod cluster).
// As a proxy for both of these, we check that the the initial Tiltfile execution
// has completed with some Kubernetes resources.
func (c *ModeController) supportsLocalMetricsStack(rStore store.RStore) bool {
	state := rStore.RLockState()
	defer rStore.RUnlockState()

	for _, mt := range state.Targets() {
		if mt.Manifest.IsK8s() && mt.Manifest.Source == model.ManifestSourceTiltfile {
			return true
		}
	}
	return false
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
			Mode: model.MetricsLocal,
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
		nil, nil, nil, model.PodReadinessWait, nil, nil)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("init local metrics: %v", err)
	}
	return model.Manifest{
		Name:   name,
		Source: model.ManifestSourceMetrics,
	}.WithDeployTarget(kTarget), nil
}
