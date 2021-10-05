// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//+build !wireinject

package buildcontrol

import (
	"context"

	"github.com/google/wire"
	"github.com/tilt-dev/wmclient/pkg/dirs"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// Injectors from wire.go:

func ProvideImageBuildAndDeployer(ctx context.Context, docker2 docker.Client, kClient k8s.Client, env k8s.Env, kubeContext k8s.KubeContext, clusterEnv docker.ClusterEnv, dir *dirs.TiltDevDir, clock build.Clock, kp KINDLoader, analytics2 *analytics.TiltAnalytics, ctrlclient client.Client, st store.RStore) (*ImageBuildAndDeployer, error) {
	labels := _wireLabelsValue
	dockerImageBuilder := build.NewDockerImageBuilder(docker2, labels)
	dockerBuilder := build.DefaultDockerBuilder(dockerImageBuilder)
	execCustomBuilder := build.NewExecCustomBuilder(docker2, clock)
	scheme := v1alpha1.NewScheme()
	namespace := provideFakeK8sNamespace()
	reconciler := kubernetesapply.NewReconciler(ctrlclient, kClient, scheme, dockerBuilder, kubeContext, st, namespace)
	imageBuildAndDeployer := NewImageBuildAndDeployer(dockerBuilder, execCustomBuilder, kClient, env, kubeContext, analytics2, clock, kp, ctrlclient, reconciler)
	return imageBuildAndDeployer, nil
}

var (
	_wireLabelsValue = dockerfile.Labels{}
)

func ProvideDockerComposeBuildAndDeployer(ctx context.Context, dcCli dockercompose.DockerComposeClient, dCli docker.Client, dir *dirs.TiltDevDir) (*DockerComposeBuildAndDeployer, error) {
	labels := _wireLabelsValue
	dockerImageBuilder := build.NewDockerImageBuilder(dCli, labels)
	dockerBuilder := build.DefaultDockerBuilder(dockerImageBuilder)
	clock := build.ProvideClock()
	execCustomBuilder := build.NewExecCustomBuilder(dCli, clock)
	imageBuilder := NewImageBuilder(dockerBuilder, execCustomBuilder)
	dockerComposeBuildAndDeployer := NewDockerComposeBuildAndDeployer(dcCli, dCli, imageBuilder, clock)
	return dockerComposeBuildAndDeployer, nil
}

// wire.go:

var BaseWireSet = wire.NewSet(wire.Value(dockerfile.Labels{}), v1alpha1.NewScheme, k8s.ProvideMinikubeClient, build.DefaultDockerBuilder, build.NewDockerImageBuilder, build.NewExecCustomBuilder, wire.Bind(new(build.CustomBuilder), new(*build.ExecCustomBuilder)), wire.Bind(new(build.DockerKubeConnection), new(build.DockerBuilder)), NewDockerComposeBuildAndDeployer,
	NewImageBuildAndDeployer,
	NewLiveUpdateBuildAndDeployer,
	NewLocalTargetBuildAndDeployer, containerupdate.NewDockerUpdater, containerupdate.NewExecUpdater, NewImageBuilder, tracer.InitOpenTelemetry, ProvideUpdateMode,
)

func provideFakeK8sNamespace() k8s.Namespace {
	return "default"
}
