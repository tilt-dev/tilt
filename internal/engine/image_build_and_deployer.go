package engine

import (
	context "context"
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/pkg/errors"

	"github.com/docker/distribution/reference"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
	"github.com/windmilleng/wmclient/pkg/analytics"
	"k8s.io/api/core/v1"
)

type deployableImageManifest interface {
	K8sYAML() string
	ManifestName() model.ManifestName
	DockerRef() reference.Named
}

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type ImageBuildAndDeployer struct {
	b             build.ImageBuilder
	cacheBuilder  build.CacheBuilder
	k8sClient     k8s.Client
	env           k8s.Env
	analytics     analytics.Analytics
	updateMode    UpdateMode
	injectSynclet bool
}

func NewImageBuildAndDeployer(
	b build.ImageBuilder,
	cacheBuilder build.CacheBuilder,
	k8sClient k8s.Client,
	env k8s.Env,
	analytics analytics.Analytics,
	updateMode UpdateMode) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		b:            b,
		cacheBuilder: cacheBuilder,
		k8sClient:    k8sClient,
		env:          env,
		analytics:    analytics,
		updateMode:   updateMode,
	}
}

// Turn on synclet injection. Should be called before any builds.
func (ibd *ImageBuildAndDeployer) SetInjectSynclet(inject bool) {
	ibd.injectSynclet = inject
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (br store.BuildResult, err error) {
	if manifest.DcYaml != "" {
		return store.BuildResult{}, CantHandleFailure{fmt.Errorf("dc manifest")}
	}
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
	ps := build.NewPipelineState(ctx, 2)
	defer func() { ps.End(ctx, err) }()

	err = manifest.ValidateKubernetesBuildAndDeploy()
	if err != nil {
		return store.BuildResult{}, err
	}

	ref, err := ibd.build(ctx, manifest, state, ps)
	if err != nil {
		return store.BuildResult{}, err
	}

	k8sEntities, namespace, err := ibd.deploy(ctx, ps, manifest, ref)
	if err != nil {
		return store.BuildResult{}, err
	}

	return store.BuildResult{
		Image:     ref,
		Namespace: namespace,
		Entities:  k8sEntities,
	}, nil
}

func (ibd *ImageBuildAndDeployer) build(ctx context.Context, manifest model.Manifest, state store.BuildState, ps *build.PipelineState) (reference.NamedTagged, error) {
	var n reference.NamedTagged

	name := manifest.DockerRef()
	if manifest.IsStaticBuild() {
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", name)
		defer ps.EndPipelineStep(ctx)

		cacheRef, err := ibd.fetchCache(ctx, manifest)
		if err != nil {
			return nil, err
		}

		df := ibd.staticDockerfile(manifest, cacheRef)
		ref, err := ibd.b.BuildDockerfile(ctx, ps, name, df, manifest.StaticBuildPath, ignore.CreateBuildContextFilter(manifest), manifest.StaticBuildArgs)

		if err != nil {
			return nil, err
		}
		n = ref

		go ibd.maybeCreateCacheFrom(ctx, ref, state, manifest, cacheRef)

	} else if !state.HasImage() || ibd.updateMode == UpdateModeNaive {
		// No existing image to build off of, need to build from scratch
		ps.StartPipelineStep(ctx, "Building from scratch: [%s]", name)
		defer ps.EndPipelineStep(ctx)

		cacheRef, err := ibd.fetchCache(ctx, manifest)
		if err != nil {
			return nil, err
		}

		df := ibd.baseDockerfile(manifest, cacheRef)
		steps := manifest.Steps
		ref, err := ibd.b.BuildImageFromScratch(ctx, ps, name, df, manifest.Mounts, ignore.CreateBuildContextFilter(manifest), steps, manifest.Entrypoint)

		if err != nil {
			return nil, err
		}
		n = ref
		go ibd.maybeCreateCacheFrom(ctx, ref, state, manifest, cacheRef)

	} else {
		changed, err := state.FilesChangedSinceLastResultImage()
		if err != nil {
			return nil, err
		}

		cf, err := build.FilesToPathMappings(changed, manifest.Mounts)
		if err != nil {
			return nil, err
		}

		ps.StartPipelineStep(ctx, "Building from existing: [%s]", name)
		defer ps.EndPipelineStep(ctx)

		steps := manifest.Steps
		ref, err := ibd.b.BuildImageFromExisting(ctx, ps, state.LastResult.Image, cf, ignore.CreateBuildContextFilter(manifest), steps)
		if err != nil {
			return nil, err
		}
		n = ref
	}

	if !ibd.canSkipPush() {
		var err error
		n, err = ibd.b.PushImage(ctx, n, ps.Writer(ctx))
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}

// Returns: the entities deployed and the namespace of the pod with the given image name/tag.
func (ibd *ImageBuildAndDeployer) deploy(ctx context.Context, ps *build.PipelineState, manifest deployableImageManifest, n reference.NamedTagged) ([]k8s.K8sEntity, k8s.Namespace, error) {
	ps.StartPipelineStep(ctx, "Deploying")
	defer ps.EndPipelineStep(ctx)

	ps.StartBuildStep(ctx, "Parsing Kubernetes config YAML")

	// TODO(nick): The parsed YAML should probably be a part of the model?
	// It doesn't make much sense to re-parse it and inject labels on every deploy.
	entities, err := k8s.ParseYAMLFromString(manifest.K8sYAML())
	if err != nil {
		return nil, "", err
	}

	didReplace := false
	newK8sEntities := []k8s.K8sEntity{}
	namespace := k8s.DefaultNamespace
	for _, e := range entities {
		e, err = k8s.InjectLabels(e, []k8s.LabelPair{TiltRunLabel(), {Key: ManifestNameLabel, Value: manifest.ManifestName().String()}})
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
		return nil, "", fmt.Errorf("Docker image missing from yaml: %s", manifest.DockerRef())
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

func (ibd *ImageBuildAndDeployer) fetchCache(ctx context.Context, manifest model.Manifest) (reference.NamedTagged, error) {
	return ibd.cacheBuilder.FetchCache(ctx, manifest.DockerRef(), manifest.CachePaths())
}

func (ibd *ImageBuildAndDeployer) maybeCreateCacheFrom(ctx context.Context, sourceRef reference.NamedTagged, state store.BuildState, manifest model.Manifest, oldCacheRef reference.NamedTagged) {
	// Only create the cache the first time we build the image.
	if !state.LastResult.IsEmpty() {
		return
	}

	// Only create the cache if there is no existing cache
	if oldCacheRef != nil {
		return
	}

	baseDockerfile := dockerfile.Dockerfile(manifest.BaseDockerfile)
	if manifest.IsStaticBuild() {
		staticDockerfile := dockerfile.Dockerfile(manifest.StaticDockerfile)
		ok := true
		baseDockerfile, _, ok = staticDockerfile.SplitIntoBaseDockerfile()
		if !ok {
			return
		}
	}

	err := ibd.cacheBuilder.CreateCacheFrom(ctx, baseDockerfile, sourceRef, manifest.CachePaths(), manifest.StaticBuildArgs)
	if err != nil {
		logger.Get(ctx).Debugf("Could not create cache: %v", err)
	}
}

func (ibd *ImageBuildAndDeployer) staticDockerfile(manifest model.Manifest, cacheRef reference.NamedTagged) dockerfile.Dockerfile {
	df := dockerfile.Dockerfile(manifest.StaticDockerfile)
	if cacheRef == nil {
		return df
	}

	if len(manifest.CachePaths()) == 0 {
		return df
	}

	_, restDf, ok := df.SplitIntoBaseDockerfile()
	if !ok {
		return df
	}

	// Replace all the lines before the ADD with a load from the Tilt cache.
	return dockerfile.FromExisting(cacheRef).
		WithLabel(build.CacheImage, "0"). // sadly there's no way to unset a label :sob:
		Append(restDf)
}

func (ibd *ImageBuildAndDeployer) baseDockerfile(manifest model.Manifest, cacheRef reference.NamedTagged) dockerfile.Dockerfile {
	df := dockerfile.Dockerfile(manifest.BaseDockerfile)
	if cacheRef == nil {
		return df
	}

	if len(manifest.CachePaths()) == 0 {
		return df
	}

	// Use the cache as the new base dockerfile.
	return dockerfile.FromExisting(cacheRef).
		WithLabel(build.CacheImage, "0") // sadly there's no way to unset a label :sob:
}
