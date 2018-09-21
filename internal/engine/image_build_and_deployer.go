package engine

import (
	context "context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/wmclient/pkg/analytics"
	"k8s.io/api/core/v1"
)

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type ImageBuildAndDeployer struct {
	b         build.ImageBuilder
	k8sClient k8s.Client
	env       k8s.Env
	analytics analytics.Analytics
}

func NewImageBuildAndDeployer(b build.ImageBuilder, k8sClient k8s.Client, env k8s.Env, analytics analytics.Analytics) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		b:         b,
		k8sClient: k8sClient,
		env:       env,
		analytics: analytics,
	}
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state BuildState) (br BuildResult, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ImageBuildAndDeployer-BuildAndDeploy")
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		incremental := "0"
		if state.HasImage() {
			incremental = "1"
		}
		tags := map[string]string{"incremental": incremental}
		ibd.analytics.Timer("build.image", time.Since(startTime), tags)
	}()

	// TODO - currently hardcoded that we have 2 pipeline steps. This might end up being dynamic? drop it from the output?
	output.Get(ctx).StartPipeline(2)
	defer func() { output.Get(ctx).EndPipeline(err) }()

	err = manifest.Validate()
	if err != nil {
		return BuildResult{}, err
	}

	ref, err := ibd.build(ctx, manifest, state)
	if err != nil {
		return BuildResult{}, err
	}

	k8sEntities, err := ibd.deploy(ctx, manifest, ref)
	if err != nil {
		return BuildResult{}, err
	}

	return BuildResult{
		Image:    ref,
		Entities: k8sEntities,
	}, nil
}

func (ibd *ImageBuildAndDeployer) build(ctx context.Context, manifest model.Manifest, state BuildState) (reference.NamedTagged, error) {
	var n reference.NamedTagged
	if !state.HasImage() {
		// No existing image to build off of, need to build from scratch
		name := manifest.DockerfileTag
		output.Get(ctx).StartPipelineStep("Building from scratch: [%s]", manifest.DockerfileTag)
		defer output.Get(ctx).EndPipelineStep()

		df := build.Dockerfile(manifest.DockerfileText)
		steps := manifest.Steps
		ref, err := ibd.b.BuildImageFromScratch(ctx, name, df, manifest.Mounts, steps, manifest.Entrypoint)

		if err != nil {
			return nil, err
		}
		n = ref

	} else {
		changed, err := state.FilesChangedSinceLastResultImage()
		if err != nil {
			return nil, err
		}

		cf, err := build.FilesToPathMappings(changed, manifest.Mounts)
		if err != nil {
			return nil, err
		}

		output.Get(ctx).StartPipelineStep("Building from existing: [%s]", manifest.DockerfileTag)
		defer output.Get(ctx).EndPipelineStep()

		steps := manifest.Steps
		ref, err := ibd.b.BuildImageFromExisting(ctx, state.LastResult.Image, cf, steps)
		if err != nil {
			return nil, err
		}
		n = ref
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

func (ibd *ImageBuildAndDeployer) deploy(ctx context.Context, manifest model.Manifest, n reference.NamedTagged) ([]k8s.K8sEntity, error) {
	output.Get(ctx).StartPipelineStep("Deploying")
	defer output.Get(ctx).EndPipelineStep()

	output.Get(ctx).StartBuildStep("Parsing Kubernetes config YAML")
	entities, err := k8s.ParseYAMLFromString(manifest.K8sYaml)
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
		if ibd.canSkipPush() {
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
		return nil, fmt.Errorf("Docker image missing from yaml: %s", manifest.DockerfileTag)
	}

	err = k8s.Update(ctx, ibd.k8sClient, newK8sEntities)
	if err != nil {
		return nil, err
	}
	return newK8sEntities, nil
}

// If we're using docker-for-desktop as our k8s backend,
// we don't need to push to the central registry.
// The k8s will use the image already available
// in the local docker daemon.
func (ibd *ImageBuildAndDeployer) canSkipPush() bool {
	return ibd.env == k8s.EnvDockerDesktop || ibd.env == k8s.EnvMinikube
}

func (ibd *ImageBuildAndDeployer) PostProcessBuild(ctx context.Context, result BuildResult) {
	// No-op: ImageBuildAndDeployer doesn't currently need any extra info for a given build result.
	return
}
