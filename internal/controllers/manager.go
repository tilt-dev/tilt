package controllers

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/store"
)

type TiltServerControllerManager struct {
	config  *rest.Config
	scheme  *runtime.Scheme
	builder cluster.ClientBuilder
	manager ctrl.Manager
	cancel  context.CancelFunc
}

var _ store.SetUpper = &TiltServerControllerManager{}
var _ store.Subscriber = &TiltServerControllerManager{}
var _ store.TearDowner = &TiltServerControllerManager{}

func NewTiltServerControllerManager(config *server.APIServerConfig, scheme *runtime.Scheme, builder cluster.ClientBuilder) (*TiltServerControllerManager, error) {
	return &TiltServerControllerManager{
		config:  config.GenericConfig.LoopbackClientConfig,
		scheme:  scheme,
		builder: builder,
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

func (m *TiltServerControllerManager) SetUp(ctx context.Context, st store.RStore) error {
	ctx, m.cancel = context.WithCancel(ctx)

	// controller-runtime internals don't really make use of verbosity levels, so in lieu of a better
	// all its logs are redirected to klog, for which there is already special handling
	// V(3) was picked because while controller-runtime is a bit chatty at startup, once steady state
	// is reached, most of the logging is generally useful (e.g. reconciler errors)
	ctrl.SetLogger(klogr.New().V(3).WithName("tilt"))

	mgr, err := ctrl.NewManager(m.config, ctrl.Options{
		Scheme: m.scheme,
		// controller manager lazily listens on a port if a webhook is registered; this functionality
		// is currently NOT used; to prevent it from listening on a default port (9443) and potentially
		// causing conflicts running multiple instances of tilt, this is set to an invalid value
		Port: -1,
		// disable metrics + health probe by setting them to "0"
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
		// leader election is unnecessary as a single manager instance is run in-process with
		// the apiserver
		LeaderElection:   false,
		LeaderElectionID: "tilt-apiserver-ctrl",

		ClientBuilder: m.builder,
	})
	if err != nil {
		return fmt.Errorf("unable to create controller manager: %v", err)
	}

	go func() {
		if err := mgr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			err = fmt.Errorf("controller manager stopped unexpectedly: %v", err)
			st.Dispatch(store.NewErrorAction(err))
		}
	}()

	m.manager = mgr

	return nil
}

func (m *TiltServerControllerManager) TearDown(_ context.Context) {
	if m.cancel != nil {
		m.cancel()
	}
}

// OnChange is a no-op but used to get initialized in upper along with the API server
func (m *TiltServerControllerManager) OnChange(_ context.Context, _ store.RStore, _ store.ChangeSummary) {
}
