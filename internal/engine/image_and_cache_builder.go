package engine

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
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

func (icb *imageAndCacheBuilder) Build(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, ps *build.PipelineState) (reference.NamedTagged, error) {
	var n reference.NamedTagged

	userFacingRefName := iTarget.ConfigurationRef.String()
	refToBuild := iTarget.DeploymentRef
	cacheInputs := icb.createCacheInputs(iTarget)
	cacheRef, err := icb.cb.FetchCache(ctx, cacheInputs)
	if err != nil {
		return nil, err
	}

	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)

		df := icb.dockerfile(iTarget, cacheRef)
		ref, err := icb.ib.BuildImage(ctx, ps, refToBuild, df, bd.BuildPath,
			ignore.CreateBuildContextFilter(iTarget), bd.BuildArgs, bd.TargetStage)

		if err != nil {
			return nil, err
		}
		n = ref

		go icb.maybeCreateCacheFrom(ctx, cacheInputs, ref, state, iTarget, cacheRef)
	case model.FastBuild:
		ps.StartPipelineStep(ctx, "Building from scratch: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)

		df := icb.baseDockerfile(bd, cacheRef, iTarget.CachePaths())
		runs := bd.Runs
		ref, err := icb.ib.DeprecatedFastBuildImage(ctx, ps, refToBuild, df, bd.Syncs, ignore.CreateBuildContextFilter(iTarget), runs, bd.Entrypoint)

		if err != nil {
			return nil, err
		}
		n = ref
		go icb.maybeCreateCacheFrom(ctx, cacheInputs, ref, state, iTarget, cacheRef)
	case model.CustomBuild:
		ps.StartPipelineStep(ctx, "Building Custom Build: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)
		ref, err := icb.custb.Build(ctx, refToBuild, bd.Command, bd.Tag)
		if err != nil {
			return nil, err
		}
		n = ref
	default:
		// Theoretically this should never trip b/c we `validate` the manifest beforehand...?
		// If we get here, something is very wrong.
		return nil, fmt.Errorf("image %q has no valid buildDetails (neither DockerBuildInfo nor FastBuildInfo)", iTarget.ConfigurationRef)
	}

	return n, nil
}

func (icb *imageAndCacheBuilder) dockerfile(image model.ImageTarget, cacheRef reference.NamedTagged) dockerfile.Dockerfile {
	df := dockerfile.Dockerfile(image.DockerBuildInfo().Dockerfile)
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
	baseDockerfile := dockerfile.Dockerfile(iTarget.TopFastBuildInfo().BaseDockerfile)
	if dbInfo, ok := iTarget.BuildDetails.(model.DockerBuild); ok {
		df := dockerfile.Dockerfile(dbInfo.Dockerfile)
		var ok bool
		baseDockerfile, _, ok = df.SplitIntoBaseDockerfile()
		if !ok {
			return build.CacheInputs{}
		}
	}

	return build.CacheInputs{
		Ref:            iTarget.ConfigurationRef.AsNamedOnly(),
		CachePaths:     iTarget.CachePaths(),
		BaseDockerfile: baseDockerfile,
	}
}

func (icb *imageAndCacheBuilder) maybeCreateCacheFrom(ctx context.Context, cacheInputs build.CacheInputs, sourceRef reference.NamedTagged, state store.BuildState, image model.ImageTarget, oldCacheRef reference.NamedTagged) {
	// Only create the cache the first time we build the image.
	if state.LastResult != nil {
		return
	}

	// Only create the cache if there is no existing cache
	if oldCacheRef != nil {
		return
	}

	var buildArgs model.DockerBuildArgs
	if dbInfo, ok := image.BuildDetails.(model.DockerBuild); ok {
		buildArgs = dbInfo.BuildArgs
	}

	err := icb.cb.CreateCacheFrom(ctx, cacheInputs, sourceRef, buildArgs)
	if err != nil {
		logger.Get(ctx).Debugf("Could not create cache: %v", err)
	}
}
