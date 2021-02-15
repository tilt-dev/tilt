package controllers

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/tilt-dev/tilt/internal/store"
	corev1alpha1 "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type TiltServerControllerManager struct {
	config *rest.Config
	cancel context.CancelFunc
}

var _ store.SetUpper = &TiltServerControllerManager{}
var _ store.Subscriber = &TiltServerControllerManager{}
var _ store.TearDowner = &TiltServerControllerManager{}

func ProvideRESTConfig(apiserverHost model.WebHost, apiserverPort model.WebPort) *rest.Config {
	return &rest.Config{
		Host: fmt.Sprintf("http://%s:%d", string(apiserverHost), int(apiserverPort)),
	}
}

func NewTiltServerControllerManager(config *rest.Config) *TiltServerControllerManager {
	return &TiltServerControllerManager{
		config: config,
	}
}

func (m *TiltServerControllerManager) SetUp(ctx context.Context, st store.RStore) error {
	if m.config == nil {
		// TODO(milas): remove this once tests use in-memory connection + real config
		logger.Get(ctx).Debugf("No REST config provided; controller manager will not be started")
		return nil
	}

	scheme := runtime.NewScheme()

	ctx, m.cancel = context.WithCancel(ctx)

	// TODO(milas): we should provide a logr.Logger facade for our logger rather than using zap
	w := logger.Get(ctx).Writer(logger.DebugLvl)
	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(w)))

	utilruntime.Must(corev1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(m.config, ctrl.Options{
		Scheme:           scheme,
		Port:             9443,
		LeaderElection:   false,
		LeaderElectionID: "b69659b9.",
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %v", err)
	}

	go func() {
		if err := mgr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			st.Dispatch(store.NewErrorAction(err))
		}
	}()

	return nil
}

func (m *TiltServerControllerManager) TearDown(_ context.Context) {
	if m.cancel != nil {
		m.cancel()
	}
}

// OnChange is a no-op but used to get initialized in upper along with the API server
func (m *TiltServerControllerManager) OnChange(_ context.Context, _ store.RStore) {}
