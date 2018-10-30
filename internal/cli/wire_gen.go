// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package cli

import (
	"context"
	"github.com/google/go-cloud/wire"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/demo"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/hud"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"time"
)

// Injectors from wire.go:

func wireDemo(ctx context.Context, branch demo.RepoBranch) (demo.Script, error) {
	env, err := k8s.DetectEnv()
	if err != nil {
		return demo.Script{}, err
	}
	config, err := k8s.ProvideRESTConfig()
	if err != nil {
		return demo.Script{}, err
	}
	coreV1Interface, err := k8s.ProvideRESTClient(config)
	if err != nil {
		return demo.Script{}, err
	}
	portForwarder := k8s.ProvidePortForwarder()
	k8sClient := k8s.NewK8sClient(ctx, env, coreV1Interface, config, portForwarder)
	reducer := _wireReducerValue
	storeStore := store.NewStore(reducer)
	deployDiscovery := engine.NewDeployDiscovery(k8sClient, storeStore)
	syncletManager := engine.NewSyncletManager(k8sClient)
	syncletBuildAndDeployer := engine.NewSyncletBuildAndDeployer(deployDiscovery, syncletManager)
	dockerCli, err := docker.DefaultDockerClient(ctx, env)
	if err != nil {
		return demo.Script{}, err
	}
	containerUpdater := build.NewContainerUpdater(dockerCli)
	analytics, err := provideAnalytics()
	if err != nil {
		return demo.Script{}, err
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
		return demo.Script{}, err
	}
	imageBuildAndDeployer := engine.NewImageBuildAndDeployer(imageBuilder, k8sClient, env, analytics, updateMode)
	buildOrder := engine.DefaultBuildOrder(syncletBuildAndDeployer, localContainerBuildAndDeployer, imageBuildAndDeployer, env, updateMode)
	fallbackTester := engine.DefaultShouldFallBack()
	compositeBuildAndDeployer := engine.NewCompositeBuildAndDeployer(buildOrder, fallbackTester)
	v := provideClock()
	renderer := hud.NewRenderer(v)
	headsUpDisplay, err := hud.NewDefaultHeadsUpDisplay(renderer)
	if err != nil {
		return demo.Script{}, err
	}
	podWatcher := engine.NewPodWatcher(k8sClient)
	serviceWatcher := engine.NewServiceWatcher(k8sClient)
	podLogManager := engine.NewPodLogManager(k8sClient)
	portForwardController := engine.NewPortForwardController(k8sClient)
	fsWatcherMaker := engine.ProvideFsWatcherMaker()
	timerMaker := engine.ProvideTimerMaker()
	watchManager := engine.NewWatchManager(fsWatcherMaker, timerMaker)
	buildController := engine.NewBuildController(compositeBuildAndDeployer)
	imageReaper := build.NewImageReaper(dockerCli)
	imageController := engine.NewImageController(imageReaper)
	upper := engine.NewUpper(ctx, compositeBuildAndDeployer, headsUpDisplay, podWatcher, serviceWatcher, storeStore, podLogManager, portForwardController, watchManager, fsWatcherMaker, buildController, imageController)
	script := demo.NewScript(upper, headsUpDisplay, k8sClient, env, storeStore, branch)
	return script, nil
}

var (
	_wireReducerValue = engine.UpperReducer
	_wireLabelsValue  = build.Labels{}
)

func wireUpper(ctx context.Context) (engine.Upper, error) {
	env, err := k8s.DetectEnv()
	if err != nil {
		return engine.Upper{}, err
	}
	config, err := k8s.ProvideRESTConfig()
	if err != nil {
		return engine.Upper{}, err
	}
	coreV1Interface, err := k8s.ProvideRESTClient(config)
	if err != nil {
		return engine.Upper{}, err
	}
	portForwarder := k8s.ProvidePortForwarder()
	k8sClient := k8s.NewK8sClient(ctx, env, coreV1Interface, config, portForwarder)
	reducer := _wireReducerValue
	storeStore := store.NewStore(reducer)
	deployDiscovery := engine.NewDeployDiscovery(k8sClient, storeStore)
	syncletManager := engine.NewSyncletManager(k8sClient)
	syncletBuildAndDeployer := engine.NewSyncletBuildAndDeployer(deployDiscovery, syncletManager)
	dockerCli, err := docker.DefaultDockerClient(ctx, env)
	if err != nil {
		return engine.Upper{}, err
	}
	containerUpdater := build.NewContainerUpdater(dockerCli)
	analytics, err := provideAnalytics()
	if err != nil {
		return engine.Upper{}, err
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
		return engine.Upper{}, err
	}
	imageBuildAndDeployer := engine.NewImageBuildAndDeployer(imageBuilder, k8sClient, env, analytics, updateMode)
	buildOrder := engine.DefaultBuildOrder(syncletBuildAndDeployer, localContainerBuildAndDeployer, imageBuildAndDeployer, env, updateMode)
	fallbackTester := engine.DefaultShouldFallBack()
	compositeBuildAndDeployer := engine.NewCompositeBuildAndDeployer(buildOrder, fallbackTester)
	v := provideClock()
	renderer := hud.NewRenderer(v)
	headsUpDisplay, err := hud.NewDefaultHeadsUpDisplay(renderer)
	if err != nil {
		return engine.Upper{}, err
	}
	podWatcher := engine.NewPodWatcher(k8sClient)
	serviceWatcher := engine.NewServiceWatcher(k8sClient)
	podLogManager := engine.NewPodLogManager(k8sClient)
	portForwardController := engine.NewPortForwardController(k8sClient)
	fsWatcherMaker := engine.ProvideFsWatcherMaker()
	timerMaker := engine.ProvideTimerMaker()
	watchManager := engine.NewWatchManager(fsWatcherMaker, timerMaker)
	buildController := engine.NewBuildController(compositeBuildAndDeployer)
	imageReaper := build.NewImageReaper(dockerCli)
	imageController := engine.NewImageController(imageReaper)
	upper := engine.NewUpper(ctx, compositeBuildAndDeployer, headsUpDisplay, podWatcher, serviceWatcher, storeStore, podLogManager, portForwardController, watchManager, fsWatcherMaker, buildController, imageController)
	return upper, nil
}

func wireK8sClient(ctx context.Context) (k8s.Client, error) {
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
	return k8sClient, nil
}

// wire.go:

var K8sWireSet = wire.NewSet(k8s.DetectEnv, k8s.ProvidePortForwarder, k8s.ProvideRESTClient, k8s.ProvideRESTConfig, k8s.NewK8sClient, wire.Bind(new(k8s.Client), k8s.K8sClient{}))

var BaseWireSet = wire.NewSet(
	K8sWireSet, docker.DefaultDockerClient, wire.Bind(new(docker.DockerClient), new(docker.DockerCli)), build.NewImageReaper, engine.DeployerWireSet, engine.DefaultShouldFallBack, engine.NewPodLogManager, engine.NewPortForwardController, engine.NewBuildController, engine.NewPodWatcher, engine.NewServiceWatcher, engine.NewImageController, provideClock, hud.NewRenderer, hud.NewDefaultHeadsUpDisplay, engine.NewUpper, provideAnalytics,
	provideUpdateModeFlag, engine.NewWatchManager, engine.ProvideFsWatcherMaker, engine.ProvideTimerMaker,
)

func provideClock() func() time.Time {
	return time.Now
}
