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
	v1 "k8s.io/api/core/v1"
)

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type ImageBuildAndDeployer struct {
	b             build.ImageBuilder
	cacheBuilder  build.CacheBuilder
	k8sClient     k8s.Client
	env           k8s.Env
	analytics     analytics.Analytics
	updateMode    UpdateMode
	injectSynclet bool
	clock         build.Clock
}

func NewImageBuildAndDeployer(
	b build.ImageBuilder,
	cacheBuilder build.CacheBuilder,
	k8sClient k8s.Client,
	env k8s.Env,
	analytics analytics.Analytics,
	updateMode UpdateMode,
	c build.Clock) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		b:            b,
		cacheBuilder: cacheBuilder,
		k8sClient:    k8sClient,
		env:          env,
		analytics:    analytics,
		updateMode:   updateMode,
		clock:        c,
	}
}

// Turn on synclet injection. Should be called before any builds.
func (ibd *ImageBuildAndDeployer) SetInjectSynclet(inject bool) {
	ibd.injectSynclet = inject
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, manifest model.Manifest, state store.BuildState) (br store.BuildResult, err error) {
	if manifest.IsDC() {
		return store.BuildResult{}, RedirectToNextBuilderf("dc manifest")
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

	numStages := 2
	ps := build.NewPipelineState(ctx, numStages, ibd.clock)
	defer func() { ps.End(ctx, err) }()

	err = manifest.ValidateDockerK8sManifest()
	if err != nil {
		return store.BuildResult{}, err
	}

	ref, err := ibd.build(ctx, manifest.ImageTarget, state, ps)
	if err != nil {
		return store.BuildResult{}, err
	}

	if !manifest.IsK8s() {
		// If a non-yaml manifest reaches this code, something is wrong.
		// If we change BaD structure such that that might reasonably happen,
		// this should be a `RedirectToNextBuilder` error.
		return store.BuildResult{}, fmt.Errorf("manifest %s has no k8s deploy info", manifest.Name)
	}

	err = ibd.deploy(ctx, ps, manifest.K8sTarget(), ref)
	if err != nil {
		return store.BuildResult{}, err
	}

	return store.BuildResult{
		Image: ref,
	}, nil
}

func (ibd *ImageBuildAndDeployer) build(ctx context.Context, image model.ImageTarget, state store.BuildState, ps *build.PipelineState) (reference.NamedTagged, error) {
	var n reference.NamedTagged

	ref := image.Ref
	cacheRef, err := ibd.fetchCache(ctx, ref, image.CachePaths())
	if err != nil {
		return nil, err
	}

	switch bd := image.BuildDetails.(type) {
	case model.StaticBuild:
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", ref)
		defer ps.EndPipelineStep(ctx)

		df := ibd.staticDockerfile(image, cacheRef)
		ref, err := ibd.b.BuildDockerfile(ctx, ps, ref, df, bd.BuildPath, ignore.CreateBuildContextFilter(image), bd.BuildArgs)

		if err != nil {
			return nil, err
		}
		n = ref

		go ibd.maybeCreateCacheFrom(ctx, ref, state, image, cacheRef)
	case model.FastBuild:
		if !state.HasImage() || ibd.updateMode == UpdateModeNaive {
			// No existing image to build off of, need to build from scratch
			ps.StartPipelineStep(ctx, "Building from scratch: [%s]", ref)
			defer ps.EndPipelineStep(ctx)

			df := ibd.baseDockerfile(bd, cacheRef, image.CachePaths())
			steps := bd.Steps
			ref, err := ibd.b.BuildImageFromScratch(ctx, ps, ref, df, bd.Mounts, ignore.CreateBuildContextFilter(image), steps, bd.Entrypoint)

			if err != nil {
				return nil, err
			}
			n = ref
			go ibd.maybeCreateCacheFrom(ctx, ref, state, image, cacheRef)

		} else {
			// We have an existing image, can do an iterative build
			changed, err := state.FilesChangedSinceLastResultImage()
			if err != nil {
				return nil, err
			}

			cf, err := build.FilesToPathMappings(changed, bd.Mounts)
			if err != nil {
				return nil, err
			}

			ps.StartPipelineStep(ctx, "Building from existing: [%s]", ref)
			defer ps.EndPipelineStep(ctx)

			steps := bd.Steps
			ref, err := ibd.b.BuildImageFromExisting(ctx, ps, state.LastResult.Image, cf, ignore.CreateBuildContextFilter(image), steps)
			if err != nil {
				return nil, err
			}
			n = ref
		}
	default:
		// Theoretically this should never trip b/c we `validate` the manifest beforehand...?
		// If we get here, something is very wrong.
		return nil, fmt.Errorf("image %q has no valid buildDetails (neither StaticBuildInfo nor FastBuildInfo)", image.Ref)
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
func (ibd *ImageBuildAndDeployer) deploy(ctx context.Context, ps *build.PipelineState, k8sInfo model.K8sTarget, ref reference.NamedTagged) error {

	ps.StartPipelineStep(ctx, "Deploying")
	defer ps.EndPipelineStep(ctx)

	ps.StartBuildStep(ctx, "Parsing Kubernetes config YAML")

	// TODO(nick): The parsed YAML should probably be a part of the model?
	// It doesn't make much sense to re-parse it and inject labels on every deploy.
	entities, err := k8s.ParseYAMLFromString(k8sInfo.YAML)
	if err != nil {
		return err
	}

	replacedAny := false
	newK8sEntities := []k8s.K8sEntity{}
	for _, e := range entities {
		e, err = k8s.InjectLabels(e, []k8s.LabelPair{TiltRunLabel(), {Key: ManifestNameLabel, Value: k8sInfo.Name.String()}})
		if err != nil {
			return errors.Wrap(err, "deploy")
		}

		// For development, image pull policy should never be set to "Always",
		// even if it might make sense to use "Always" in prod. People who
		// set "Always" for development are shooting their own feet.
		e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
		if err != nil {
			return err
		}

		// When working with a local k8s cluster, we set the pull policy to Never,
		// to ensure that k8s fails hard if the image is missing from docker.
		policy := v1.PullIfNotPresent
		if ibd.canSkipPush() {
			policy = v1.PullNever
		}
		if ref != nil {
			var replaced bool
			e, replaced, err = k8s.InjectImageDigest(e, ref, policy)
			if err != nil {
				return err
			}
			if replaced {
				replacedAny = true

				if ibd.injectSynclet {
					var sidecarInjected bool
					e, sidecarInjected, err = sidecar.InjectSyncletSidecar(e, ref)
					if err != nil {
						return err
					}
					if !sidecarInjected {
						return fmt.Errorf("Could not inject synclet: %v", e)
					}
				}
			}
		}

		newK8sEntities = append(newK8sEntities, e)
	}

	if ref != nil && !replacedAny {
		return fmt.Errorf("Docker image missing from yaml: %s", ref)
	}

	err = ibd.k8sClient.Upsert(ctx, newK8sEntities)
	if err != nil {
		return err
	}
	return nil
}

// If we're using docker-for-desktop as our k8s backend,
// we don't need to push to the central registry.
// The k8s will use the image already available
// in the local docker daemon.
func (ibd *ImageBuildAndDeployer) canSkipPush() bool {
	return ibd.env.IsLocalCluster()
}

func (ibd *ImageBuildAndDeployer) fetchCache(ctx context.Context, ref reference.Named, cachePaths []string) (reference.NamedTagged, error) {
	return ibd.cacheBuilder.FetchCache(ctx, ref, cachePaths)
}

func (ibd *ImageBuildAndDeployer) maybeCreateCacheFrom(ctx context.Context, sourceRef reference.NamedTagged, state store.BuildState, image model.ImageTarget, oldCacheRef reference.NamedTagged) {
	// Only create the cache the first time we build the image.
	if !state.LastResult.IsEmpty() {
		return
	}

	// Only create the cache if there is no existing cache
	if oldCacheRef != nil {
		return
	}

	baseDockerfile := dockerfile.Dockerfile(image.FastBuildInfo().BaseDockerfile)
	var buildArgs model.DockerBuildArgs

	if sbInfo, ok := image.BuildDetails.(model.StaticBuild); ok {
		staticDockerfile := dockerfile.Dockerfile(sbInfo.Dockerfile)
		ok := true
		baseDockerfile, _, ok = staticDockerfile.SplitIntoBaseDockerfile()
		if !ok {
			return
		}

		buildArgs = sbInfo.BuildArgs
	}

	err := ibd.cacheBuilder.CreateCacheFrom(ctx, baseDockerfile, sourceRef,
		image.CachePaths(), buildArgs)
	if err != nil {
		logger.Get(ctx).Debugf("Could not create cache: %v", err)
	}
}

func (ibd *ImageBuildAndDeployer) staticDockerfile(image model.ImageTarget, cacheRef reference.NamedTagged) dockerfile.Dockerfile {
	df := dockerfile.Dockerfile(image.StaticBuildInfo().Dockerfile)
	if cacheRef == nil {
		return df
	}

	if len(image.CachePaths()) == 0 {
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

func (ibd *ImageBuildAndDeployer) baseDockerfile(fbInfo model.FastBuild,
	cacheRef reference.NamedTagged, cachePaths []string) dockerfile.Dockerfile {
	df := dockerfile.Dockerfile(fbInfo.BaseDockerfile)
	if cacheRef == nil {
		return df
	}

	if len(cachePaths) == 0 {
		return df
	}

	// Use the cache as the new base dockerfile.
	return dockerfile.FromExisting(cacheRef).
		WithLabel(build.CacheImage, "0") // sadly there's no way to unset a label :sob:
}
