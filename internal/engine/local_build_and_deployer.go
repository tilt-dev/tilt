package engine

import (
	context "context"
	"fmt"

	"github.com/docker/distribution/reference"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/image"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
	"k8s.io/api/core/v1"
)

var _ BuildAndDeployer = localBuildAndDeployer{}

type localBuildAndDeployer struct {
	b         build.ImageBuilder
	history   image.ImageHistory
	k8sClient k8s.Client
	env       k8s.Env
}

func NewLocalBuildAndDeployer(b build.ImageBuilder, k8sClient k8s.Client, history image.ImageHistory, env k8s.Env) (BuildAndDeployer, error) {
	return localBuildAndDeployer{
		b:         b,
		history:   history,
		k8sClient: k8sClient,
		env:       env,
	}, nil
}

func (l localBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, token *buildToken, changedFiles []string) (*buildToken, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-localBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	// TODO - currently hardcoded that we have 2 pipeline steps. This might end up being dynamic? drop it from the output?
	output.Get(ctx).StartPipeline(2)
	defer output.Get(ctx).EndPipeline()

	err := service.Validate()
	if err != nil {
		return nil, err
	}

	ref, err := l.build(ctx, service, token, changedFiles)
	if err != nil {
		return nil, err
	}

	k8sEntities, err := l.deploy(ctx, service, ref)
	if err != nil {
		return nil, err
	}

	newToken := &buildToken{n: ref, entities: k8sEntities}
	return newToken, err
}

func (l localBuildAndDeployer) build(ctx context.Context, service model.Service, token *buildToken, changedFiles []string) (reference.NamedTagged, error) {
	checkpoint := l.history.CheckpointNow()
	var n reference.NamedTagged
	if token.isEmpty() {
		name, err := reference.ParseNormalizedNamed(service.DockerfileTag)
		if err != nil {
			return nil, err
		}
		output.Get(ctx).StartPipelineStep("Building from scratch: [%s]", service.DockerfileTag)
		defer output.Get(ctx).EndPipelineStep()

		ref, err := l.b.BuildImageFromScratch(ctx, name, build.Dockerfile(service.DockerfileText), service.Mounts, service.Steps, service.Entrypoint)

		if err != nil {
			return nil, err
		}
		n = ref

	} else {
		cf, err := build.FilesToPathMappings(changedFiles, service.Mounts)
		if err != nil {
			return nil, err
		}

		output.Get(ctx).StartPipelineStep("Building from existing: [%s]", service.DockerfileTag)
		defer output.Get(ctx).EndPipelineStep()

		ref, err := l.b.BuildImageFromExisting(ctx, token.n, cf, service.Steps)
		if err != nil {
			return nil, err
		}
		n = ref
	}

	logger.Get(ctx).Verbosef("(Adding checkpoint to history)")
	err := l.history.AddAndPersist(ctx, n, checkpoint, service)
	if err != nil {
		return nil, err
	}

	if !l.canSkipPush() {
		var err error
		n, err = l.b.PushImage(ctx, n)
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}

func (l localBuildAndDeployer) deploy(ctx context.Context, service model.Service, n reference.NamedTagged) ([]k8s.K8sEntity, error) {
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
		if l.canSkipPush() {
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

	err = k8s.Update(ctx, l.k8sClient, newK8sEntities)
	if err != nil {
		return nil, err
	}
	return newK8sEntities, nil
}

// If we're using docker-for-desktop as our k8s backend,
// we don't need to push to the central registry.
// The k8s will use the image already available
// in the local docker daemon.
func (l localBuildAndDeployer) canSkipPush() bool {
	return l.env == k8s.EnvDockerDesktop || l.env == k8s.EnvMinikube
}
