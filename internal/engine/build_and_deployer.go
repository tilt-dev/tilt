package engine

import (
	"context"
	"fmt"

	"github.com/windmilleng/tilt/internal/output"
	"k8s.io/api/core/v1"

	"github.com/docker/distribution/reference"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/image"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type buildToken struct {
	n reference.NamedTagged
}

func (b *buildToken) isEmpty() bool {
	return b == nil
}

type BuildAndDeployer interface {
	// Builds and deployed the specified service.
	// Returns a buildToken that can be passed on successive calls to allow incremental builds.
	// If buildToken is passed and changedFiles is non-nil, changedFiles should specify the list of files that have
	//   changed since the last build.
	BuildAndDeploy(ctx context.Context, service model.Service, token *buildToken, changedFiles []string) (*buildToken, error)
}

var _ BuildAndDeployer = localBuildAndDeployer{}

type localBuildAndDeployer struct {
	b         build.Builder
	history   image.ImageHistory
	k8sClient k8s.Client
	env       k8s.Env
}

func NewLocalBuildAndDeployer(b build.Builder, k8sClient k8s.Client, history image.ImageHistory, env k8s.Env) (BuildAndDeployer, error) {
	return localBuildAndDeployer{
		b:         b,
		history:   history,
		k8sClient: k8sClient,
		env:       env,
	}, nil
}

func (l localBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, token *buildToken, changedFiles []string) (*buildToken, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-BuildAndDeploy")
	defer span.Finish()

	// TODO - currently hardcoded that we have 2 pipeline steps. This might end up being dynamic? drop it from the output?
	output.Get(ctx).StartPipeline(2)
	defer output.Get(ctx).EndPipeline()

	// TODO(dmiller) add back history
	//checkpoint := l.history.CheckpointNow()
	err := service.Validate()
	if err != nil {
		return nil, err
	}

	var n reference.NamedTagged
	if token.isEmpty() {
		name, err := reference.ParseNormalizedNamed(service.DockerfileTag)
		if err != nil {
			return nil, err
		}
		output.Get(ctx).StartPipelineStep("Building from scratch: [%s]", service.DockerfileTag)
		newDigest, err := l.b.BuildDockerFromScratch(ctx, name, build.Dockerfile(service.DockerfileText), service.Mounts, service.Steps, service.Entrypoint)
		output.Get(ctx).EndPipelineStep()
		if err != nil {
			return nil, err
		}
		n = newDigest

	} else {
		cf, err := build.FilesToPathMappings(changedFiles, service.Mounts)
		if err != nil {
			return nil, err
		}

		output.Get(ctx).StartPipelineStep("Building from existing: [%s]", service.DockerfileTag)
		newDigest, err := l.b.BuildDockerFromExisting(ctx, token.n, cf, service.Steps)
		output.Get(ctx).EndPipelineStep()
		if err != nil {
			return nil, err
		}
		n = newDigest
	}

	logger.Get(ctx).Verbosef("(Adding checkpoint to history)")
	// TODO(dmiller) add back history
	// err = l.history.AddAndPersist(ctx, name, d, checkpoint, service)
	// if err != nil {
	// 	return nil, err
	// }

	// If we're using docker-for-desktop as our k8s backend,
	// we don't need to push to the central registry.
	// The k8s will use the image already available
	// in the local docker daemon.
	canSkipPush := l.env == k8s.EnvDockerDesktop || l.env == k8s.EnvMinikube
	if !canSkipPush {
		n, err = l.b.PushDocker(ctx, n)
		if err != nil {
			return nil, err
		}
	}

	output.Get(ctx).StartPipelineStep("Deploying")
	defer output.Get(ctx).EndPipelineStep()

	output.Get(ctx).StartBuildStep("Parsing Kubernetes config YAML")
	entities, err := k8s.ParseYAMLFromString(service.K8sYaml)
	if err != nil {
		return nil, err
	}

	didReplace := false
	newK8sEntities := []k8s.K8sEntity{}
	for _, e := range entities {
		// For development, image pull policy should never be set to "Always",
		// even if it might make sense to use "Always" in prod. People who
		// set "Always" for development are shooting their own feet.
		e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
		if err != nil {
			return nil, err
		}

		// When working with a local k8s cluster, we set the pull policy to Never,
		// to ensure that k8s fails hard if the image is missing from docker.
		policy := v1.PullIfNotPresent
		if canSkipPush {
			policy = v1.PullNever
		}
		e, replaced, err := k8s.InjectImageDigest(e, n, policy)
		if err != nil {
			return nil, err
		}
		if replaced {
			didReplace = true
		}
		newK8sEntities = append(newK8sEntities, e)
	}

	if !didReplace {
		return nil, fmt.Errorf("Docker image missing from yaml: %s", service.DockerfileTag)
	}

	newToken := &buildToken{n}
	err = k8s.Update(ctx, l.k8sClient, newK8sEntities)
	return newToken, err
}
