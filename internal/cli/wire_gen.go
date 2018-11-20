// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package cli

import (
	context "context"
	wire "github.com/google/go-cloud/wire"
	build "github.com/windmilleng/tilt/internal/build"
	demo "github.com/windmilleng/tilt/internal/demo"
	docker "github.com/windmilleng/tilt/internal/docker"
	dockerfile "github.com/windmilleng/tilt/internal/dockerfile"
	engine "github.com/windmilleng/tilt/internal/engine"
	hud "github.com/windmilleng/tilt/internal/hud"
	k8s "github.com/windmilleng/tilt/internal/k8s"
	store "github.com/windmilleng/tilt/internal/store"
	time "time"
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
	deployDiscovery := engine.NewDeployDiscovery(k8sClient)
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
	cacheBuilder := build.NewCacheBuilder(dockerCli)
	updateModeFlag2 := provideUpdateModeFlag()
	updateMode, err := engine.ProvideUpdateMode(updateModeFlag2, env)
	if err != nil {
		return demo.Script{}, err
	}
	imageBuildAndDeployer := engine.NewImageBuildAndDeployer(imageBuilder, cacheBuilder, k8sClient, env, analytics, updateMode)
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
	nodeIP, err := k8s.DetectNodeIP(ctx, env)
	if err != nil {
		return demo.Script{}, err
	}
	serviceWatcher := engine.NewServiceWatcher(k8sClient, nodeIP)
	reducer := _wireReducerValue
	logActionsFlag2 := provideLogActions()
	store2 := store.NewStore(reducer, logActionsFlag2)
	podLogManager := engine.NewPodLogManager(k8sClient)
	portForwardController := engine.NewPortForwardController(k8sClient)
	fsWatcherMaker := engine.ProvideFsWatcherMaker()
	timerMaker := engine.ProvideTimerMaker()
	watchManager := engine.NewWatchManager(fsWatcherMaker, timerMaker)
	buildController := engine.NewBuildController(compositeBuildAndDeployer)
	imageReaper := build.NewImageReaper(dockerCli)
	imageController := engine.NewImageController(imageReaper)
	globalYAMLBuildController := engine.NewGlobalYAMLBuildController(k8sClient)
	tiltfileController := engine.NewTiltfileController()
	upper := engine.NewUpper(ctx, compositeBuildAndDeployer, headsUpDisplay, podWatcher, serviceWatcher, store2, podLogManager, portForwardController, watchManager, fsWatcherMaker, buildController, imageController, globalYAMLBuildController, tiltfileController)
	script := demo.NewScript(upper, headsUpDisplay, k8sClient, env, store2, branch)
	return script, nil
}

var (
	_wireLabelsValue  = dockerfile.Labels{}
	_wireReducerValue = engine.UpperReducer
)

func wireHudAndUpper(ctx context.Context) (HudAndUpper, error) {
	v := provideClock()
	renderer := hud.NewRenderer(v)
	headsUpDisplay, err := hud.NewDefaultHeadsUpDisplay(renderer)
	if err != nil {
		return HudAndUpper{}, err
	}
	env, err := k8s.DetectEnv()
	if err != nil {
		return HudAndUpper{}, err
	}
	config, err := k8s.ProvideRESTConfig()
	if err != nil {
		return HudAndUpper{}, err
	}
	coreV1Interface, err := k8s.ProvideRESTClient(config)
	if err != nil {
		return HudAndUpper{}, err
	}
	portForwarder := k8s.ProvidePortForwarder()
	k8sClient := k8s.NewK8sClient(ctx, env, coreV1Interface, config, portForwarder)
	deployDiscovery := engine.NewDeployDiscovery(k8sClient)
	syncletManager := engine.NewSyncletManager(k8sClient)
	syncletBuildAndDeployer := engine.NewSyncletBuildAndDeployer(deployDiscovery, syncletManager)
	dockerCli, err := docker.DefaultDockerClient(ctx, env)
	if err != nil {
		return HudAndUpper{}, err
	}
	containerUpdater := build.NewContainerUpdater(dockerCli)
	analytics, err := provideAnalytics()
	if err != nil {
		return HudAndUpper{}, err
	}
	localContainerBuildAndDeployer := engine.NewLocalContainerBuildAndDeployer(containerUpdater, analytics, deployDiscovery)
	console := build.DefaultConsole()
	writer := build.DefaultOut()
	labels := _wireLabelsValue
	dockerImageBuilder := build.NewDockerImageBuilder(dockerCli, console, writer, labels)
	imageBuilder := build.DefaultImageBuilder(dockerImageBuilder)
	cacheBuilder := build.NewCacheBuilder(dockerCli)
	updateModeFlag2 := provideUpdateModeFlag()
	updateMode, err := engine.ProvideUpdateMode(updateModeFlag2, env)
	if err != nil {
		return HudAndUpper{}, err
	}
	imageBuildAndDeployer := engine.NewImageBuildAndDeployer(imageBuilder, cacheBuilder, k8sClient, env, analytics, updateMode)
	buildOrder := engine.DefaultBuildOrder(syncletBuildAndDeployer, localContainerBuildAndDeployer, imageBuildAndDeployer, env, updateMode)
	fallbackTester := engine.DefaultShouldFallBack()
	compositeBuildAndDeployer := engine.NewCompositeBuildAndDeployer(buildOrder, fallbackTester)
	podWatcher := engine.NewPodWatcher(k8sClient)
	nodeIP, err := k8s.DetectNodeIP(ctx, env)
	if err != nil {
		return HudAndUpper{}, err
	}
	serviceWatcher := engine.NewServiceWatcher(k8sClient, nodeIP)
	reducer := _wireReducerValue
	logActionsFlag2 := provideLogActions()
	store2 := store.NewStore(reducer, logActionsFlag2)
	podLogManager := engine.NewPodLogManager(k8sClient)
	portForwardController := engine.NewPortForwardController(k8sClient)
	fsWatcherMaker := engine.ProvideFsWatcherMaker()
	timerMaker := engine.ProvideTimerMaker()
	watchManager := engine.NewWatchManager(fsWatcherMaker, timerMaker)
	buildController := engine.NewBuildController(compositeBuildAndDeployer)
	imageReaper := build.NewImageReaper(dockerCli)
	imageController := engine.NewImageController(imageReaper)
	globalYAMLBuildController := engine.NewGlobalYAMLBuildController(k8sClient)
	tiltfileController := engine.NewTiltfileController()
	upper := engine.NewUpper(ctx, compositeBuildAndDeployer, headsUpDisplay, podWatcher, serviceWatcher, store2, podLogManager, portForwardController, watchManager, fsWatcherMaker, buildController, imageController, globalYAMLBuildController, tiltfileController)
	hudAndUpper := provideHudAndUpper(headsUpDisplay, upper)
	return hudAndUpper, nil
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

var K8sWireSet = wire.NewSet(k8s.DetectEnv, k8s.DetectNodeIP, k8s.ProvidePortForwarder, k8s.ProvideRESTClient, k8s.ProvideRESTConfig, k8s.NewK8sClient, wire.Bind(new(k8s.Client), k8s.K8sClient{}))

var BaseWireSet = wire.NewSet(
	K8sWireSet, docker.DefaultDockerClient, wire.Bind(new(docker.DockerClient), new(docker.DockerCli)), build.NewImageReaper, engine.DeployerWireSet, engine.DefaultShouldFallBack, engine.NewPodLogManager, engine.NewPortForwardController, engine.NewBuildController, engine.NewPodWatcher, engine.NewServiceWatcher, engine.NewImageController, engine.NewTiltfileController, provideClock, hud.NewRenderer, hud.NewDefaultHeadsUpDisplay, provideLogActions, store.NewStore, engine.NewUpper, provideAnalytics,
	provideUpdateModeFlag, engine.NewWatchManager, engine.ProvideFsWatcherMaker, engine.ProvideTimerMaker, provideHudAndUpper,
)

type HudAndUpper struct {
	hud   hud.HeadsUpDisplay
	upper engine.Upper
}

func provideHudAndUpper(h hud.HeadsUpDisplay, upper engine.Upper) HudAndUpper {
	return HudAndUpper{h, upper}
}

func provideClock() func() time.Time {
	return time.Now
}
