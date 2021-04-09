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
	NewImageBuildAndDeployer,
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
	dir *dirs.TiltDevDir,
	clock build.Clock,
	kp KINDLoader,
	analytics *analytics.TiltAnalytics) (*ImageBuildAndDeployer, error) {
	wire.Build(
		BaseWireSet,
		wire.Value(UpdateModeFlag(UpdateModeAuto)),
		k8s.ProvideContainerRuntime,
	)

	return nil, nil
}
