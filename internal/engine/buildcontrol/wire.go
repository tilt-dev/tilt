// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package buildcontrol

import (
	"context"

	"github.com/google/wire"
	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tracer"
)

var BaseWireSet = wire.NewSet(
	// dockerImageBuilder ( = ImageBuilder)
	wire.Value(dockerfile.Labels{}),

	k8s.ProvideMinikubeClient,
	build.DefaultDockerBuilder,
	build.NewDockerImageBuilder,
	build.NewExecCustomBuilder,
	wire.Bind(new(build.CustomBuilder), new(*build.ExecCustomBuilder)),

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
	analytics *analytics.TiltAnalytics) (*ImageBuildAndDeployer, error) {
	wire.Build(
		BaseWireSet,
		wire.Value(UpdateModeFlag(UpdateModeAuto)),
	)

	return nil, nil
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
