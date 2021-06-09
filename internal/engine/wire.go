// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package engine

import (
	"context"

	"github.com/google/wire"
	"github.com/tilt-dev/wmclient/pkg/dirs"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s"
)

var DeployerBaseWireSet = wire.NewSet(
	buildcontrol.BaseWireSet,
	wire.Value(UpperReducer),

	// BuildOrder
	DefaultBuildOrder,

	wire.Bind(new(buildcontrol.BuildAndDeployer), new(*CompositeBuildAndDeployer)),
	NewCompositeBuildAndDeployer,
)

var DeployerWireSetTest = wire.NewSet(
	DeployerBaseWireSet,
	wire.InterfaceValue(new(sdktrace.SpanProcessor), (sdktrace.SpanProcessor)(nil)),
)

var DeployerWireSet = wire.NewSet(
	DeployerBaseWireSet,
)

func provideFakeBuildAndDeployer(
	ctx context.Context,
	docker docker.Client,
	kClient k8s.Client,
	dir *dirs.TiltDevDir,
	env k8s.Env,
	updateMode buildcontrol.UpdateModeFlag,
	dcc dockercompose.DockerComposeClient,
	clock build.Clock,
	kp buildcontrol.KINDLoader,
	analytics *analytics.TiltAnalytics) (buildcontrol.BuildAndDeployer, error) {
	wire.Build(
		DeployerWireSetTest,
		k8s.ProvideContainerRuntime,
		provideFakeKubeContext,
		provideFakeDockerClusterEnv,
	)

	return nil, nil
}

func provideFakeKubeContext(env k8s.Env) k8s.KubeContext {
	return k8s.KubeContext(string(env))
}

// A simplified version of the normal calculation we do
// about whether we can build direct to a cluser
func provideFakeDockerClusterEnv(c docker.Client, k8sEnv k8s.Env, kubeContext k8s.KubeContext, runtime container.Runtime) docker.ClusterEnv {
	env := c.Env()
	isDockerRuntime := runtime == container.RuntimeDocker
	isLocalDockerCluster := k8sEnv == k8s.EnvMinikube || k8sEnv == k8s.EnvMicroK8s || k8sEnv == k8s.EnvDockerDesktop
	if isDockerRuntime && isLocalDockerCluster {
		env.BuildToKubeContexts = append(env.BuildToKubeContexts, string(kubeContext))
	}

	fake, ok := c.(*docker.FakeClient)
	if ok {
		fake.FakeEnv = env
	}

	return docker.ClusterEnv(env)
}
