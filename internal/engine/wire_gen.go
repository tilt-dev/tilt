// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package engine

import (
	"context"

	"github.com/google/wire"
	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/containerupdate"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/minikube"
	"github.com/windmilleng/tilt/internal/mode"
	"github.com/windmilleng/tilt/internal/synclet"
)

// Injectors from wire.go:

func provideBuildAndDeployer(ctx context.Context, docker2 docker.Client, kClient k8s.Client, dir *dirs.WindmillDir, env k8s.Env, updateMode mode.UpdateModeFlag, sCli synclet.SyncletClient, dcc dockercompose.DockerComposeClient, clock build.Clock, kp KINDPusher, analytics2 *analytics.TiltAnalytics) (BuildAndDeployer, error) {
	syncletManager := containerupdate.NewSyncletManagerForTests(kClient, sCli)
	runtime := k8s.ProvideContainerRuntime(ctx, kClient)
	modeUpdateMode, err := mode.ProvideUpdateMode(updateMode, env, runtime)
	if err != nil {
		return nil, err
	}
	k8sContainerUpdater := containerupdate.ProvideK8sContainerUpdater(kClient, docker2, syncletManager, env, modeUpdateMode, runtime)
	engineK8sLiveUpdBAD := ProvideLiveUpdateBuildAndDeployerForK8s(k8sContainerUpdater, env)
	labels := _wireLabelsValue
	dockerImageBuilder := build.NewDockerImageBuilder(docker2, labels)
	imageBuilder := build.DefaultImageBuilder(dockerImageBuilder)
	cacheBuilder := build.NewCacheBuilder(docker2)
	execCustomBuilder := build.NewExecCustomBuilder(docker2, clock)
	imageBuildAndDeployer := NewImageBuildAndDeployer(imageBuilder, cacheBuilder, execCustomBuilder, kClient, env, analytics2, modeUpdateMode, clock, runtime, kp)
	engineImageAndCacheBuilder := NewImageAndCacheBuilder(imageBuilder, cacheBuilder, execCustomBuilder, modeUpdateMode)
	dockerComposeBuildAndDeployer := NewDockerComposeBuildAndDeployer(dcc, docker2, engineImageAndCacheBuilder, clock)
	k8sOrder := DefaultBuildOrderForK8s(engineK8sLiveUpdBAD, imageBuildAndDeployer, dockerComposeBuildAndDeployer, modeUpdateMode)
	dcContainerUpdater := containerupdate.ProvideDCContainerUpdater(docker2, modeUpdateMode)
	engineDcLiveUpdBAD := ProvideLiveUpdateBuildAndDeployerForDC(dcContainerUpdater, env)
	dcOrder := DefaultBuildOrderForDC(engineDcLiveUpdBAD, imageBuildAndDeployer, dockerComposeBuildAndDeployer, modeUpdateMode)
	compositeBuildAndDeployer := NewCompositeBuildAndDeployer(k8sOrder, dcOrder)
	return compositeBuildAndDeployer, nil
}

var (
	_wireLabelsValue = dockerfile.Labels{}
)

func provideImageBuildAndDeployer(ctx context.Context, docker2 docker.Client, kClient k8s.Client, env k8s.Env, dir *dirs.WindmillDir, clock build.Clock, kp KINDPusher, analytics2 *analytics.TiltAnalytics) (*ImageBuildAndDeployer, error) {
	labels := _wireLabelsValue
	dockerImageBuilder := build.NewDockerImageBuilder(docker2, labels)
	imageBuilder := build.DefaultImageBuilder(dockerImageBuilder)
	cacheBuilder := build.NewCacheBuilder(docker2)
	execCustomBuilder := build.NewExecCustomBuilder(docker2, clock)
	updateModeFlag := _wireUpdateModeFlagValue
	runtime := k8s.ProvideContainerRuntime(ctx, kClient)
	updateMode, err := mode.ProvideUpdateMode(updateModeFlag, env, runtime)
	if err != nil {
		return nil, err
	}
	imageBuildAndDeployer := NewImageBuildAndDeployer(imageBuilder, cacheBuilder, execCustomBuilder, kClient, env, analytics2, updateMode, clock, runtime, kp)
	return imageBuildAndDeployer, nil
}

var (
	_wireUpdateModeFlagValue = mode.UpdateModeFlag(mode.UpdateModeAuto)
)

func provideDockerComposeBuildAndDeployer(ctx context.Context, dcCli dockercompose.DockerComposeClient, dCli docker.Client, dir *dirs.WindmillDir) (*DockerComposeBuildAndDeployer, error) {
	labels := _wireLabelsValue
	dockerImageBuilder := build.NewDockerImageBuilder(dCli, labels)
	imageBuilder := build.DefaultImageBuilder(dockerImageBuilder)
	cacheBuilder := build.NewCacheBuilder(dCli)
	clock := build.ProvideClock()
	execCustomBuilder := build.NewExecCustomBuilder(dCli, clock)
	updateModeFlag := _wireModeUpdateModeFlagValue
	env := _wireEnvValue
	portForwarder := k8s.ProvidePortForwarder()
	clientConfig := k8s.ProvideClientConfig()
	namespace := k8s.ProvideConfigNamespace(clientConfig)
	config, err := k8s.ProvideKubeConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	kubeContext, err := k8s.ProvideKubeContext(config)
	if err != nil {
		return nil, err
	}
	int2 := provideKubectlLogLevelInfo()
	kubectlRunner := k8s.ProvideKubectlRunner(kubeContext, int2)
	client := k8s.ProvideK8sClient(ctx, env, portForwarder, namespace, kubectlRunner, clientConfig)
	runtime := k8s.ProvideContainerRuntime(ctx, client)
	updateMode, err := mode.ProvideUpdateMode(updateModeFlag, env, runtime)
	if err != nil {
		return nil, err
	}
	engineImageAndCacheBuilder := NewImageAndCacheBuilder(imageBuilder, cacheBuilder, execCustomBuilder, updateMode)
	dockerComposeBuildAndDeployer := NewDockerComposeBuildAndDeployer(dcCli, dCli, engineImageAndCacheBuilder, clock)
	return dockerComposeBuildAndDeployer, nil
}

var (
	_wireModeUpdateModeFlagValue = mode.UpdateModeFlag(mode.UpdateModeAuto)
	_wireEnvValue                = k8s.Env(k8s.EnvNone)
)

// wire.go:

var DeployerBaseWireSet = wire.NewSet(wire.Value(dockerfile.Labels{}), wire.Value(UpperReducer), minikube.ProvideMinikubeClient, build.DefaultImageBuilder, build.NewCacheBuilder, build.NewDockerImageBuilder, build.NewExecCustomBuilder, wire.Bind(new(build.CustomBuilder), new(build.ExecCustomBuilder)), NewImageBuildAndDeployer, containerupdate.ProvideK8sContainerUpdater, containerupdate.ProvideDCContainerUpdater, NewSyncletBuildAndDeployer,
	ProvideLiveUpdateBuildAndDeployerForK8s,
	ProvideLiveUpdateBuildAndDeployerForDC,
	NewDockerComposeBuildAndDeployer,
	NewImageAndCacheBuilder,
	DefaultBuildOrderForK8s,
	DefaultBuildOrderForDC, wire.Bind(new(BuildAndDeployer), new(CompositeBuildAndDeployer)), NewCompositeBuildAndDeployer, mode.ProvideUpdateMode,
)

var DeployerWireSetTest = wire.NewSet(
	DeployerBaseWireSet, containerupdate.NewSyncletManagerForTests,
)

var DeployerWireSet = wire.NewSet(
	DeployerBaseWireSet, containerupdate.NewSyncletManager,
)

func provideKubectlLogLevelInfo() k8s.KubectlLogLevel {
	return k8s.KubectlLogLevel(logger.InfoLvl)
}
