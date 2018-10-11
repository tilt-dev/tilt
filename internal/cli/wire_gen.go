// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package cli

import (
	"context"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

// Injectors from wire.go:

func wireManifestCreator(ctx context.Context, browser engine.BrowserMode) (model.ManifestCreator, error) {
	env, err := k8s.DetectEnv()
	if err != nil {
		return nil, err
	}
	config, err := k8s.ProvideRESTConfig()
	if err != nil {
		return nil, err
	}
	coreV1Interface, err := k8s.ProvideRESTClient(config)
	if err != nil {
		return nil, err
	}
	portForwarder := k8s.ProvidePortForwarder()
	k8sClient := k8s.NewK8sClient(ctx, env, coreV1Interface, config, portForwarder)
	deployDiscovery := engine.NewDeployDiscovery(k8sClient)
	syncletManager := engine.NewSyncletManager(k8sClient)
	syncletBuildAndDeployer := engine.NewSyncletBuildAndDeployer(deployDiscovery, syncletManager)
	dockerCli, err := docker.DefaultDockerClient(ctx, env)
	if err != nil {
		return nil, err
	}
	containerUpdater := build.NewContainerUpdater(dockerCli)
	analytics, err := provideAnalytics()
	if err != nil {
		return nil, err
	}
	localContainerBuildAndDeployer := engine.NewLocalContainerBuildAndDeployer(containerUpdater, analytics, deployDiscovery)
	console := build.DefaultConsole()
	writer := build.DefaultOut()
	labels := _wireLabelsValue
	dockerImageBuilder := build.NewDockerImageBuilder(dockerCli, console, writer, labels)
	imageBuilder := build.DefaultImageBuilder(dockerImageBuilder)
	engineUpdateModeFlag := provideUpdateModeFlag()
	updateMode, err := engine.ProvideUpdateMode(engineUpdateModeFlag, env)
	if err != nil {
		return nil, err
	}
	imageBuildAndDeployer := engine.NewImageBuildAndDeployer(imageBuilder, k8sClient, env, analytics, updateMode)
	buildOrder := engine.DefaultBuildOrder(syncletBuildAndDeployer, localContainerBuildAndDeployer, imageBuildAndDeployer, env, updateMode)
	fallbackTester := engine.DefaultShouldFallBack()
	compositeBuildAndDeployer := engine.NewCompositeBuildAndDeployer(buildOrder, fallbackTester)
	imageReaper := build.NewImageReaper(dockerCli)
	headsUpDisplay, err := hud.NewDefaultHeadsUpDisplay()
	if err != nil {
		return nil, err
	}
	podWatcherMaker := engine.ProvidePodWatcherMaker(k8sClient)
	storeStore := store.NewStore()
	upper := engine.NewUpper(ctx, compositeBuildAndDeployer, k8sClient, browser, imageReaper, headsUpDisplay, podWatcherMaker, storeStore)
	return upper, nil
}

var (
	_wireLabelsValue = build.Labels{}
)
