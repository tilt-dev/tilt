package engine

import (
	context "context"
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/store"

	"github.com/pkg/errors"

	"github.com/docker/distribution/reference"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
	"github.com/windmilleng/wmclient/pkg/analytics"
	"k8s.io/api/core/v1"
)

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type ImageBuildAndDeployer struct {
	b             build.ImageBuilder
	k8sClient     k8s.Client
	env           k8s.Env
	analytics     analytics.Analytics
	updateMode    UpdateMode
	injectSynclet bool
}

func NewImageBuildAndDeployer(b build.ImageBuilder, k8sClient k8s.Client, env k8s.Env, analytics analytics.Analytics, updateMode UpdateMode) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		b:          b,
		k8sClient:  k8sClient,
		env:        env,
		analytics:  analytics,
		updateMode: updateMode,
	}
}

// Turn on synclet injection. Should be called before any builds.
func (ibd *ImageBuildAndDeployer) SetInjectSynclet(inject bool) {
	ibd.injectSynclet = inject
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (br store.BuildResult, err error) {
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
		return store.BuildResult{}, err
	}

	ref, err := ibd.build(ctx, manifest, state)
	if err != nil {
		return store.BuildResult{}, err
	}

	k8sEntities, namespace, err := ibd.deploy(ctx, manifest, ref)
	if err != nil {
		return store.BuildResult{}, err
	}

	return store.BuildResult{
		Image:     ref,
		Namespace: namespace,
		Entities:  k8sEntities,
	}, nil
}

func (ibd *ImageBuildAndDeployer) build(ctx context.Context, manifest model.Manifest, state store.BuildState) (reference.NamedTagged, error) {
	var n reference.NamedTagged

	name := manifest.DockerRef
	if manifest.IsStaticBuild() {
		output.Get(ctx).StartPipelineStep("Building Dockerfile: [%s]", name)
		defer output.Get(ctx).EndPipelineStep()

		df := build.Dockerfile(manifest.StaticDockerfile)
		ref, err := ibd.b.BuildDockerfile(ctx, name, df, manifest.StaticBuildPath, ignore.CreateBuildContextFilter(manifest))

		if err != nil {
			return nil, err
		}
		n = ref

	} else if !state.HasImage() || ibd.updateMode == UpdateModeNaive {
		// No existing image to build off of, need to build from scratch
		output.Get(ctx).StartPipelineStep("Building from scratch: [%s]", name)
		defer output.Get(ctx).EndPipelineStep()

		df := build.Dockerfile(manifest.BaseDockerfile)
		steps := manifest.Steps
		ref, err := ibd.b.BuildImageFromScratch(ctx, name, df, manifest.Mounts, ignore.CreateBuildContextFilter(manifest), steps, manifest.Entrypoint)

		if err != nil {
			return nil, err
		}
		n = ref

	} else {
		changed, err := state.FilesChangedSinceLastResultImage()
		if err != nil {
			return nil, err
		}

		cf := build.FilesToPathMappings(ctx, changed, manifest.Mounts)

		output.Get(ctx).StartPipelineStep("Building from existing: [%s]", name)
		defer output.Get(ctx).EndPipelineStep()

		steps := manifest.Steps
		ref, err := ibd.b.BuildImageFromExisting(ctx, state.LastResult.Image, cf, ignore.CreateBuildContextFilter(manifest), steps)
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

// Returns: the entities deployed and the namespace of the pod with the given image name/tag.
func (ibd *ImageBuildAndDeployer) deploy(ctx context.Context, manifest model.Manifest, n reference.NamedTagged) ([]k8s.K8sEntity, k8s.Namespace, error) {
	output.Get(ctx).StartPipelineStep("Deploying")
	defer output.Get(ctx).EndPipelineStep()

	output.Get(ctx).StartBuildStep("Parsing Kubernetes config YAML")

	// TODO(nick): The parsed YAML should probably be a part of the model?
	// It doesn't make much sense to re-parse it and inject labels on every deploy.
	entities, err := k8s.ParseYAMLFromString(manifest.K8sYaml)
	if err != nil {
		return nil, "", err
	}

	didReplace := false
	newK8sEntities := []k8s.K8sEntity{}
	namespace := k8s.DefaultNamespace
	for _, e := range entities {
		e, err = k8s.InjectLabels(e, []k8s.LabelPair{TiltRunLabel(), {Key: ManifestNameLabel, Value: manifest.Name.String()}})
		if err != nil {
			return nil, "", errors.Wrap(err, "deploy")
		}

		// For development, image pull policy should never be set to "Always",
		// even if it might make sense to use "Always" in prod. People who
		// set "Always" for development are shooting their own feet.
		e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
		if err != nil {
			return nil, "", err
		}

		// When working with a local k8s cluster, we set the pull policy to Never,
		// to ensure that k8s fails hard if the image is missing from docker.
		policy := v1.PullIfNotPresent
		if ibd.canSkipPush() {
			policy = v1.PullNever
		}
		e, replaced, err := k8s.InjectImageDigest(e, n, policy)
		if err != nil {
			return nil, "", err
		}
		if replaced {
			didReplace = true
			namespace = e.Namespace()

			if ibd.injectSynclet {
				e, replaced, err = sidecar.InjectSyncletSidecar(e, n)
				if err != nil {
					return nil, "", err
				} else if !replaced {
					return nil, "", fmt.Errorf("Could not inject synclet: %v", e)
				}
			}
		}

		newK8sEntities = append(newK8sEntities, e)
	}

	if !didReplace {
		return nil, "", fmt.Errorf("Docker image missing from yaml: %s", manifest.DockerRef)
	}

	err = ibd.k8sClient.Upsert(ctx, newK8sEntities)
	if err != nil {
		return nil, "", err
	}
	return newK8sEntities, namespace, nil
}

// If we're using docker-for-desktop as our k8s backend,
// we don't need to push to the central registry.
// The k8s will use the image already available
// in the local docker daemon.
func (ibd *ImageBuildAndDeployer) canSkipPush() bool {
	return ibd.env.IsLocalCluster()
}

func (ibd *ImageBuildAndDeployer) PostProcessBuild(ctx context.Context, result, previousResult store.BuildResult) {
	// No-op: ImageBuildAndDeployer doesn't currently need any extra info for a given build result.
	return
}
