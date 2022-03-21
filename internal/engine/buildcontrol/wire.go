//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package buildcontrol

import (
	"context"

	"github.com/google/wire"
	"github.com/jonboulle/clockwork"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/controllers/core/dockercomposeservice"
	"github.com/tilt-dev/tilt/internal/controllers/core/kubernetesapply"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/internal/tracer"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var BaseWireSet = wire.NewSet(
	// dockerImageBuilder ( = ImageBuilder)
	wire.Value(dockerfile.Labels{}),

	v1alpha1.NewScheme,
	k8s.ProvideMinikubeClient,
	build.NewDockerBuilder,
	build.NewCustomBuilder,
	wire.Bind(new(build.DockerKubeConnection), new(*build.DockerBuilder)),

	// BuildOrder
	NewDockerComposeBuildAndDeployer,
	NewImageBuildAndDeployer,
	NewLiveUpdateBuildAndDeployer,
	NewLocalTargetBuildAndDeployer,
	containerupdate.NewDockerUpdater,
	containerupdate.NewExecUpdater,
	NewImageBuilder,

	tracer.InitOpenTelemetry,

	liveupdates.ProvideUpdateMode,
)

func ProvideImageBuildAndDeployer(
	ctx context.Context,
	docker docker.Client,
	kClient k8s.Client,
	env clusterid.Product,
	kubeContext k8s.KubeContext,
	clusterEnv docker.ClusterEnv,
	dir *dirs.TiltDevDir,
	clock build.Clock,
	kp KINDLoader,
	analytics *analytics.TiltAnalytics,
	ctrlclient ctrlclient.Client,
	st store.RStore,
	execer localexec.Execer) (*ImageBuildAndDeployer, error) {
	wire.Build(
		BaseWireSet,
		kubernetesapply.NewReconciler,
		provideFakeK8sNamespace,
	)

	return nil, nil
}

func provideFakeK8sNamespace() k8s.Namespace {
	return "default"
}

func ProvideDockerComposeBuildAndDeployer(
	ctx context.Context,
	dcCli dockercompose.DockerComposeClient,
	dCli docker.Client,
	ctrlclient ctrlclient.Client,
	st store.RStore,
	clock clockwork.Clock,
	dir *dirs.TiltDevDir) (*DockerComposeBuildAndDeployer, error) {
	wire.Build(
		BaseWireSet,
		dockercomposeservice.WireSet,
		build.ProvideClock,
	)

	return nil, nil
}
