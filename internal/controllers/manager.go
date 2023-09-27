package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type UncachedObjects []ctrlclient.Object

func ProvideUncachedObjects() UncachedObjects {
	return nil
}

type TiltServerControllerManager struct {
	config          *rest.Config
	scheme          *runtime.Scheme
	deferredClient  *DeferredClient
	uncachedObjects UncachedObjects

	manager ctrl.Manager
	cancel  context.CancelFunc
}

var _ store.SetUpper = &TiltServerControllerManager{}
var _ store.Subscriber = &TiltServerControllerManager{}
var _ store.TearDowner = &TiltServerControllerManager{}

func NewTiltServerControllerManager(config *server.APIServerConfig, scheme *runtime.Scheme, deferredClient *DeferredClient, uncachedObjects UncachedObjects) (*TiltServerControllerManager, error) {
	return &TiltServerControllerManager{
		config:          config.GenericConfig.LoopbackClientConfig,
		scheme:          scheme,
		deferredClient:  deferredClient,
		uncachedObjects: uncachedObjects,
	}, nil
}

func (m *TiltServerControllerManager) GetManager() ctrl.Manager {
	return m.manager
}

func (m *TiltServerControllerManager) GetClient() ctrlclient.Client {
	if m.manager == nil {
		return nil
	}
	return m.manager.GetClient()
}

func (m *TiltServerControllerManager) SetUp(ctx context.Context, _ store.RStore) error {
	ctx, m.cancel = context.WithCancel(ctx)

	// controller-runtime internals don't really make use of verbosity levels, so in lieu of a better
	// mechanism, all its logs are redirected to a custom logger that filters out logs
	// we don't care about.
	logr := logr.New(&logSink{ctx: ctx, logger: logger.Get(ctx), Formatter: funcr.NewFormatter(funcr.Options{})})
	timeout := time.Duration(0)

	mgr, err := ctrl.NewManager(m.config, ctrl.Options{
		Scheme: m.scheme,

		// Disable metrics server.
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},

		// Disable health probe.
		HealthProbeBindAddress: "0",
		// leader election is unnecessary as a single manager instance is run in-process with
		// the apiserver
		LeaderElection:   false,
		LeaderElectionID: "tilt-apiserver-ctrl",

		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: m.uncachedObjects,
			},
		},

		Logger:                  logr,
		GracefulShutdownTimeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("unable to create controller manager: %v", err)
	}

	// provide the deferred client with the real client now that it has been initialized
	m.deferredClient.initialize(mgr.GetClient())
	m.manager = mgr

	return nil
}

func (m *TiltServerControllerManager) TearDown(_ context.Context) {
	if m.cancel != nil {
		m.cancel()
	}
}

// OnChange is a no-op but used to get initialized in upper along with the API server
func (m *TiltServerControllerManager) OnChange(_ context.Context, _ store.RStore, _ store.ChangeSummary) error {
	return nil
}
