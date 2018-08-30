package engine

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/image"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
	"k8s.io/api/core/v1"
	v1beta13 "k8s.io/api/extensions/v1beta1"
)

var _ BuildAndDeployer = ImageBuildAndDeployer{}

const servNameLabel = "tiltServiceName"

type ImageBuildAndDeployer struct {
	b         build.ImageBuilder
	history   image.ImageHistory
	k8sClient k8s.Client
	env       k8s.Env
}

func NewImageBuildAndDeployer(b build.ImageBuilder, k8sClient k8s.Client, history image.ImageHistory, env k8s.Env) (ImageBuildAndDeployer, error) {
	return ImageBuildAndDeployer{
		b:         b,
		history:   history,
		k8sClient: k8sClient,
		env:       env,
	}, nil
}

func (ibd ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, state BuildState) (BuildResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ImageBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	// TODO - currently hardcoded that we have 2 pipeline steps. This might end up being dynamic? drop it from the output?
	output.Get(ctx).StartPipeline(2)
	defer output.Get(ctx).EndPipeline()

	err := service.Validate()
	if err != nil {
		return BuildResult{}, err
	}

	ref, err := ibd.build(ctx, service, state)
	if err != nil {
		return BuildResult{}, err
	}

	k8sEntities, err := ibd.deploy(ctx, service, ref)
	if err != nil {
		return BuildResult{}, err
	}

	return BuildResult{
		Image:    ref,
		Entities: k8sEntities,
	}, nil
}

func (ibd ImageBuildAndDeployer) build(ctx context.Context, service model.Service, state BuildState) (reference.NamedTagged, error) {
	checkpoint := ibd.history.CheckpointNow()
	var n reference.NamedTagged
	if state.IsEmpty() {
		name, err := reference.ParseNormalizedNamed(service.DockerfileTag)
		if err != nil {
			return nil, err
		}
		output.Get(ctx).StartPipelineStep("Building from scratch: [%s]", service.DockerfileTag)
		defer output.Get(ctx).EndPipelineStep()

		ref, err := ibd.b.BuildImageFromScratch(ctx, name, build.Dockerfile(service.DockerfileText), service.Mounts, service.Steps, service.Entrypoint)

		if err != nil {
			return nil, err
		}
		n = ref

	} else {
		cf, err := build.FilesToPathMappings(state.FilesChanged(), service.Mounts)
		if err != nil {
			return nil, err
		}

		output.Get(ctx).StartPipelineStep("Building from existing: [%s]", service.DockerfileTag)
		defer output.Get(ctx).EndPipelineStep()

		ref, err := ibd.b.BuildImageFromExisting(ctx, state.LastResult.Image, cf, service.Steps)
		if err != nil {
			return nil, err
		}
		n = ref
	}

	logger.Get(ctx).Verbosef("(Adding checkpoint to history)")
	err := ibd.history.AddAndPersist(ctx, n, checkpoint, service)
	if err != nil {
		return nil, err
	}

	if !ibd.canSkipPush() {
		var err error
		n, err = ibd.b.PushImage(ctx, n)
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}

func (ibd ImageBuildAndDeployer) deploy(ctx context.Context, service model.Service, img reference.NamedTagged) ([]k8s.K8sEntity, error) {
	output.Get(ctx).StartPipelineStep("Deploying")
	defer output.Get(ctx).EndPipelineStep()

	output.Get(ctx).StartBuildStep("Parsing Kubernetes config YAML")
	entities, err := k8s.ParseYAMLFromString(service.K8sYaml)
	if err != nil {
		return nil, err
	}

	// update k8s entities with new image, labels, etc.
	newK8sEntities, err := updateK8sEntities(entities, service, img, ibd.canSkipPush())
	if err != nil {
		return nil, err
	}

	err = k8s.Update(ctx, ibd.k8sClient, newK8sEntities)
	if err != nil {
		return nil, err
	}
	return newK8sEntities, nil
}

// updateK8sEntities updates the given entities for the given service: injecting
// the newly built image, labeling pods with service name, etc.
func updateK8sEntities(entities []k8s.K8sEntity, service model.Service, img reference.NamedTagged, canSkipPush bool) ([]k8s.K8sEntity, error) {
	var newK8sEntities []k8s.K8sEntity

	injectedImg := false
	labeledWithName := false

	for _, e := range entities {
		// TODO(maia): we'll need to handle this case for any version of Deployment
		if deployment, ok := e.Obj.(*v1beta13.Deployment); ok {
			if deployment.Spec.Template.Labels == nil {
				deployment.Spec.Template.Labels = make(map[string]string)
			}

			deployment.Spec.Template.Labels[servNameLabel] = service.Name.String()
			labeledWithName = true
		}

		// For development, image pull policy should never be set to "Always",
		// even if it might make sense to use "Always" in prod. People who
		// set "Always" for development are shooting their own feet.
		e, err := k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
		if err != nil {
			return nil, err
		}

		// When working with a local k8s cluster, we set the pull policy to Never,
		// to ensure that k8s fails hard if the image is missing from docker.
		policy := v1.PullIfNotPresent
		if canSkipPush {
			policy = v1.PullNever
		}
		e, replaced, err := k8s.InjectImageDigest(e, img, policy)
		if err != nil {
			return nil, err
		}
		if replaced {
			injectedImg = true
		}
		newK8sEntities = append(newK8sEntities, e)
	}

	if !injectedImg {
		return nil, fmt.Errorf("docker image missing from yaml: %s", service.DockerfileTag)
	}
	if !labeledWithName {
		return nil, fmt.Errorf("could not tag service with label 'tiltServiceName'")
	}
	return newK8sEntities, nil
}

// If we're using docker-for-desktop as our k8s backend,
// we don't need to push to the central registry.
// The k8s will use the image already available
// in the local docker daemon.
func (ibd ImageBuildAndDeployer) canSkipPush() bool {
	return ibd.env == k8s.EnvDockerDesktop || ibd.env == k8s.EnvMinikube
}
