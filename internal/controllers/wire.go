package controllers

import (
	"github.com/google/wire"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/core/cmd"
	"github.com/tilt-dev/tilt/internal/controllers/core/extension"
	"github.com/tilt-dev/tilt/internal/controllers/core/extensionrepo"
	"github.com/tilt-dev/tilt/internal/controllers/core/filewatch"
	"github.com/tilt-dev/tilt/internal/controllers/core/kubernetesapply"
	"github.com/tilt-dev/tilt/internal/controllers/core/kubernetesdiscovery"
	"github.com/tilt-dev/tilt/internal/controllers/core/podlogstream"
	"github.com/tilt-dev/tilt/internal/controllers/core/portforward"
	"github.com/tilt-dev/tilt/internal/controllers/core/tiltfile"
	"github.com/tilt-dev/tilt/internal/controllers/core/uibutton"
	"github.com/tilt-dev/tilt/internal/controllers/core/uiresource"
	"github.com/tilt-dev/tilt/internal/controllers/core/uisession"
)

var controllerSet = wire.NewSet(
	filewatch.NewController,
	kubernetesdiscovery.NewReconciler,
	portforward.NewReconciler,
	podlogstream.NewController,
	podlogstream.NewPodSource,
	kubernetesapply.NewReconciler,

	ProvideControllers,
)

func ProvideControllers(
	fileWatch *filewatch.Controller,
	cmds *cmd.Controller,
	podlogstreams *podlogstream.Controller,
	kubernetesDiscovery *kubernetesdiscovery.Reconciler,
	kubernetesApply *kubernetesapply.Reconciler,
	uis *uisession.Reconciler,
	uir *uiresource.Reconciler,
	uib *uibutton.Reconciler,
	pfr *portforward.Reconciler,
	tfr *tiltfile.Reconciler,
	extr *extension.Reconciler,
	extrr *extensionrepo.Reconciler) []Controller {
	return []Controller{
		fileWatch,
		cmds,
		podlogstreams,
		kubernetesDiscovery,
		kubernetesApply,
		uis,
		uir,
		uib,
		pfr,
		tfr,
		extr,
		extrr,
	}
}

var WireSet = wire.NewSet(
	NewTiltServerControllerManager,

	NewControllerBuilder,
	ProvideUncachedObjects,

	ProvideDeferredClient,
	wire.Bind(new(ctrlclient.Client), new(*DeferredClient)),

	cmd.WireSet,
	controllerSet,
	uiresource.WireSet,
	uisession.WireSet,
	uibutton.WireSet,
	tiltfile.WireSet,
	extensionrepo.WireSet,
	extension.WireSet,
)
