// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package buildcontrol

import (
	"context"

	"github.com/google/wire"
	"github.com/tilt-dev/wmclient/pkg/dirs"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/controllers/core/kubernetesapply"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/tracer"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var BaseWireSet = wire.NewSet(
	// dockerImageBuilder ( = ImageBuilder)
	wire.Value(dockerfile.Labels{}),

	v1alpha1.NewScheme,
	k8s.ProvideMinikubeClient,
	build.DefaultDockerBuilder,
	build.NewDockerImageBuilder,
	build.NewExecCustomBuilder,
	wire.Bind(new(build.CustomBuilder), new(*build.ExecCustomBuilder)),
	wire.Bind(new(build.DockerKubeConnection), new(build.DockerBuilder)),

	// BuildOrder
	NewDockerComposeBuildAndDeployer,
	NewImageBuildAndDeployer,
	NewLiveUpdateBuildAndDeployer,
	NewLocalTargetBuildAndDeployer,
	containerupdate.NewDockerUpdater,
	containerupdate.NewExecUpdater,
	NewImageBuilder,

	tracer.InitOpenTelemetry,

	ProvideUpdateMode,
)

func ProvideImageBuildAndDeployer(
	ctx context.Context,
	docker docker.Client,
	kClient k8s.Client,
	env k8s.Env,
	kubeContext k8s.KubeContext,
	clusterEnv docker.ClusterEnv,
	dir *dirs.TiltDevDir,
	clock build.Clock,
	kp KINDLoader,
	analytics *analytics.TiltAnalytics,
	ctrlclient ctrlclient.Client,
	st store.RStore) (*ImageBuildAndDeployer, error) {
	wire.Build(
		BaseWireSet,
		wire.Value(UpdateModeFlag(UpdateModeAuto)),
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
	dir *dirs.TiltDevDir) (*DockerComposeBuildAndDeployer, error) {
	wire.Build(
		BaseWireSet,
		wire.Value(UpdateModeFlag(UpdateModeAuto)),
		build.ProvideClock,
		wire.Value(docker.ClusterEnv(docker.Env{})),

		// EnvNone ensures that we get an exploding k8s client.
		wire.Value(k8s.KubeContextOverride("")),
		wire.Value(k8s.NamespaceOverride("")),
		k8s.ProvideClientConfig,
		k8s.ProvideKubeContext,
		k8s.ProvideKubeConfig,
	)

	return nil, nil
}
