package build

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/mode"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type ImageAndCacheBuilder struct {
	ib         ImageBuilder
	cb         CacheBuilder
	updateMode mode.UpdateMode
}

func NewImageAndCacheBuilder(ib ImageBuilder, cb CacheBuilder, updateMode mode.UpdateMode) *ImageAndCacheBuilder {
	return &ImageAndCacheBuilder{
		ib:         ib,
		cb:         cb,
		updateMode: updateMode,
	}
}

func (icb *ImageAndCacheBuilder) Build(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, ps *PipelineState, canSkipPush bool) (reference.NamedTagged, error) {
	var n reference.NamedTagged

	ref := iTarget.Ref
	cacheRef, err := icb.cb.FetchCache(ctx, ref, iTarget.CachePaths())
	if err != nil {
		return nil, err
	}

	switch bd := iTarget.BuildDetails.(type) {
	case model.StaticBuild:
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", ref)
		defer ps.EndPipelineStep(ctx)

		df := icb.staticDockerfile(iTarget, cacheRef)
		ref, err := icb.ib.BuildDockerfile(ctx, ps, ref, df, bd.BuildPath, ignore.CreateBuildContextFilter(iTarget), bd.BuildArgs)

		if err != nil {
			return nil, err
		}
		n = ref

		go icb.maybeCreateCacheFrom(ctx, ref, state, iTarget, cacheRef)
	case model.FastBuild:
		if !state.HasImage() || icb.updateMode == mode.UpdateModeNaive {
			// No existing image to build off of, need to build from scratch
			ps.StartPipelineStep(ctx, "Building from scratch: [%s]", ref)
			defer ps.EndPipelineStep(ctx)

			df := icb.baseDockerfile(bd, cacheRef, iTarget.CachePaths())
			steps := bd.Steps
			ref, err := icb.ib.BuildImageFromScratch(ctx, ps, ref, df, bd.Mounts, ignore.CreateBuildContextFilter(iTarget), steps, bd.Entrypoint)

			if err != nil {
				return nil, err
			}
			n = ref
			go icb.maybeCreateCacheFrom(ctx, ref, state, iTarget, cacheRef)

		} else {
			// We have an existing image, can do an iterative build
			changed, err := state.FilesChangedSinceLastResultImage()
			if err != nil {
				return nil, err
			}

			cf, err := FilesToPathMappings(changed, bd.Mounts)
			if err != nil {
				return nil, err
			}

			ps.StartPipelineStep(ctx, "Building from existing: [%s]", ref)
			defer ps.EndPipelineStep(ctx)

			steps := bd.Steps
			ref, err := icb.ib.BuildImageFromExisting(ctx, ps, state.LastResult.Image, cf, ignore.CreateBuildContextFilter(iTarget), steps)
			if err != nil {
				return nil, err
			}
			n = ref
		}
	default:
		// Theoretically this should never trip b/c we `validate` the manifest beforehand...?
		// If we get here, something is very wrong.
		return nil, fmt.Errorf("image %q has no valid buildDetails (neither StaticBuildInfo nor FastBuildInfo)", iTarget.Ref)
	}

	if !canSkipPush {
		var err error
		n, err = icb.ib.PushImage(ctx, n, ps.Writer(ctx))
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}

func (icb *ImageAndCacheBuilder) staticDockerfile(image model.ImageTarget, cacheRef reference.NamedTagged) dockerfile.Dockerfile {
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
		WithLabel(CacheImage, "0"). // sadly there's no way to unset a label :sob:
		Append(restDf)
}

func (icb *ImageAndCacheBuilder) baseDockerfile(fbInfo model.FastBuild,
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
		WithLabel(CacheImage, "0") // sadly there's no way to unset a label :sob:
}

func (icb *ImageAndCacheBuilder) maybeCreateCacheFrom(ctx context.Context, sourceRef reference.NamedTagged, state store.BuildState, image model.ImageTarget, oldCacheRef reference.NamedTagged) {
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

	err := icb.cb.CreateCacheFrom(ctx, baseDockerfile, sourceRef,
		image.CachePaths(), buildArgs)
	if err != nil {
		logger.Get(ctx).Debugf("Could not create cache: %v", err)
	}
}
