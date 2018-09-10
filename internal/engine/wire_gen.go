// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package engine

import (
	context "context"
	build "github.com/windmilleng/tilt/internal/build"
	k8s "github.com/windmilleng/tilt/internal/k8s"
	dirs "github.com/windmilleng/wmclient/pkg/dirs"
)

// Injectors from wire.go:

func provideBuildAndDeployer(ctx context.Context, docker build.DockerClient, k8s2 k8s.Client, dir *dirs.WindmillDir, env k8s.Env, skipContainer bool) (BuildAndDeployer, error) {
	containerUpdater := build.NewContainerUpdater(docker)
	firstLineBuildAndDeployer := NewContainerBuildAndDeployerAsFirstLine(containerUpdater, env, k8s2, skipContainer)
	console := build.DefaultConsole()
	writer := build.DefaultOut()
	labels := _wireLabelsValue
	dockerImageBuilder := build.NewDockerImageBuilder(docker, console, writer, labels)
	imageBuilder := build.DefaultImageBuilder(dockerImageBuilder)
	fallbackBuildAndDeployer := NewImageBuildAndDeployerAsFallback(imageBuilder, k8s2, env)
	compositeBuildAndDeployer := NewCompositeBuildAndDeployer(firstLineBuildAndDeployer, fallbackBuildAndDeployer)
	return compositeBuildAndDeployer, nil
}

var (
	_wireLabelsValue = build.Labels{}
)
