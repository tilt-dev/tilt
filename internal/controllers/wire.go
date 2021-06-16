package controllers

import (
	"github.com/google/wire"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/core/cmd"
	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch"
	"github.com/tilt-dev/tilt/internal/controllers/core/kubernetesdiscovery"
	"github.com/tilt-dev/tilt/internal/controllers/core/podlogstream"
	"github.com/tilt-dev/tilt/internal/controllers/core/portforward"
	"github.com/tilt-dev/tilt/internal/controllers/core/uibutton"
	"github.com/tilt-dev/tilt/internal/controllers/core/uiresource"
	"github.com/tilt-dev/tilt/internal/controllers/core/uisession"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var controllerSet = wire.NewSet(
	filewatch.NewController,
	kubernetesdiscovery.NewReconciler,
	portforward.NewReconciler,
	podlogstream.NewController,
	podlogstream.NewPodSource,

	ProvideControllers,
)

func ProvideControllers(
	fileWatch *filewatch.Controller,
	cmds *cmd.Controller,
	podlogstreams *podlogstream.Controller,
	kubernetesDiscovery *kubernetesdiscovery.Reconciler,
	uis *uisession.Reconciler,
	uir *uiresource.Reconciler,
	uib *uibutton.Reconciler,
	pfr *portforward.Reconciler) []Controller {
	return []Controller{
		fileWatch,
		cmds,
		podlogstreams,
		kubernetesDiscovery,
		uis,
		uir,
		uib,
		pfr,
	}
}

var WireSet = wire.NewSet(
	NewTiltServerControllerManager,

	v1alpha1.NewScheme,
	NewControllerBuilder,
	ProvideUncachedObjects,

	ProvideDeferredClient,
	wire.Bind(new(ctrlclient.Client), new(*DeferredClient)),

	cmd.WireSet,
	controllerSet,
	uiresource.WireSet,
	uisession.WireSet,
	uibutton.WireSet,
)
