package engine

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type imageAndCacheBuilder struct {
	ib         build.ImageBuilder
	cb         build.CacheBuilder
	custb      build.CustomBuilder
	updateMode UpdateMode
}

func NewImageAndCacheBuilder(ib build.ImageBuilder, cb build.CacheBuilder, custb build.CustomBuilder, updateMode UpdateMode) *imageAndCacheBuilder {
	return &imageAndCacheBuilder{
		ib:         ib,
		cb:         cb,
		custb:      custb,
		updateMode: updateMode,
	}
}

func (icb *imageAndCacheBuilder) Build(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, ps *build.PipelineState, canSkipPush bool) (reference.NamedTagged, error) {
	var n reference.NamedTagged

	ref := iTarget.Ref
	cacheInputs := icb.createCacheInputs(iTarget)
	cacheRef, err := icb.cb.FetchCache(ctx, cacheInputs)
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

		go icb.maybeCreateCacheFrom(ctx, cacheInputs, ref, state, iTarget, cacheRef)
	case model.FastBuild:
		if !state.HasImage() || icb.updateMode == UpdateModeNaive {
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
			go icb.maybeCreateCacheFrom(ctx, cacheInputs, ref, state, iTarget, cacheRef)

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
			ref, err := icb.ib.BuildImageFromExisting(ctx, ps, state.LastResult.Image, cf, ignore.CreateBuildContextFilter(iTarget), steps)
			if err != nil {
				return nil, err
			}
			n = ref
		}
	case model.CustomBuild:
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", ref)
		defer ps.EndPipelineStep(ctx)
		ref, err := icb.custb.Build(ctx, ref, bd.Command)
		if err != nil {
			return nil, err
		}
		n = ref
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

func (icb *imageAndCacheBuilder) staticDockerfile(image model.ImageTarget, cacheRef reference.NamedTagged) dockerfile.Dockerfile {
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

func (icb *imageAndCacheBuilder) baseDockerfile(fbInfo model.FastBuild,
	cacheRef build.CacheRef, cachePaths []string) dockerfile.Dockerfile {
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

func (icb *imageAndCacheBuilder) createCacheInputs(iTarget model.ImageTarget) build.CacheInputs {
	baseDockerfile := dockerfile.Dockerfile(iTarget.FastBuildInfo().BaseDockerfile)
	if sbInfo, ok := iTarget.BuildDetails.(model.StaticBuild); ok {
		staticDockerfile := dockerfile.Dockerfile(sbInfo.Dockerfile)
		ok := true
		baseDockerfile, _, ok = staticDockerfile.SplitIntoBaseDockerfile()
		if !ok {
			return build.CacheInputs{}
		}
	}

	return build.CacheInputs{
		Ref:            iTarget.Ref,
		CachePaths:     iTarget.CachePaths(),
		BaseDockerfile: baseDockerfile,
	}
}

func (icb *imageAndCacheBuilder) maybeCreateCacheFrom(ctx context.Context, cacheInputs build.CacheInputs, sourceRef reference.NamedTagged, state store.BuildState, image model.ImageTarget, oldCacheRef reference.NamedTagged) {
	// Only create the cache the first time we build the image.
	if !state.LastResult.IsEmpty() {
		return
	}

	// Only create the cache if there is no existing cache
	if oldCacheRef != nil {
		return
	}

	var buildArgs model.DockerBuildArgs
	if sbInfo, ok := image.BuildDetails.(model.StaticBuild); ok {
		buildArgs = sbInfo.BuildArgs
	}

	err := icb.cb.CreateCacheFrom(ctx, cacheInputs, sourceRef, buildArgs)
	if err != nil {
		logger.Get(ctx).Debugf("Could not create cache: %v", err)
	}
}
