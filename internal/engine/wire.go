//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package engine

import (
	"context"

	"github.com/google/wire"
	"github.com/jonboulle/clockwork"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/core/cmd"
	"github.com/tilt-dev/tilt/internal/controllers/core/dockercomposeservice"
	"github.com/tilt-dev/tilt/internal/controllers/core/kubernetesapply"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
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
	wire.InterfaceValue(new(sdktrace.SpanExporter), (sdktrace.SpanExporter)(nil)),
)

var DeployerWireSet = wire.NewSet(
	DeployerBaseWireSet,
)

func provideFakeBuildAndDeployer(
	ctx context.Context,
	docker docker.Client,
	kClient k8s.Client,
	dir *dirs.TiltDevDir,
	env clusterid.Product,
	updateMode liveupdates.UpdateModeFlag,
	dcc dockercompose.DockerComposeClient,
	clock build.Clock,
	kp buildcontrol.KINDLoader,
	analytics *analytics.TiltAnalytics,
	ctrlClient ctrlclient.Client,
	st store.RStore,
	execer localexec.Execer) (buildcontrol.BuildAndDeployer, error) {
	wire.Build(
		DeployerWireSetTest,
		k8s.ProvideContainerRuntime,
		provideFakeKubeContext,
		provideFakeDockerClusterEnv,
		provideFakeK8sNamespace,
		kubernetesapply.NewReconciler,
		dockercomposeservice.WireSet,
		cmd.WireSet,
		clockwork.NewRealClock,
		provideFakeEnv,
	)

	return nil, nil
}

func provideFakeEnv() *localexec.Env {
	return localexec.EmptyEnv()
}

func provideFakeK8sNamespace() k8s.Namespace {
	return "default"
}

func provideFakeKubeContext(env clusterid.Product) k8s.KubeContext {
	return k8s.KubeContext(string(env))
}

// A simplified version of the normal calculation we do
// about whether we can build direct to a cluser
func provideFakeDockerClusterEnv(c docker.Client, k8sEnv clusterid.Product, kubeContext k8s.KubeContext, runtime container.Runtime) docker.ClusterEnv {
	env := c.Env()
	isDockerRuntime := runtime == container.RuntimeDocker
	isLocalDockerCluster := k8sEnv == clusterid.ProductMinikube || k8sEnv == clusterid.ProductMicroK8s || k8sEnv == clusterid.ProductDockerDesktop
	if isDockerRuntime && isLocalDockerCluster {
		env.BuildToKubeContexts = append(env.BuildToKubeContexts, string(kubeContext))
	}

	fake, ok := c.(*docker.FakeClient)
	if ok {
		fake.FakeEnv = env
	}

	return docker.ClusterEnv(env)
}
