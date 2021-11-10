package buildcontrol

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ImageBuilder struct {
	db    build.DockerBuilder
	custb build.CustomBuilder
}

func NewImageBuilder(db build.DockerBuilder, custb build.CustomBuilder) *ImageBuilder {
	return &ImageBuilder{
		db:    db,
		custb: custb,
	}
}

func (icb *ImageBuilder) CanReuseRef(ctx context.Context, iTarget model.ImageTarget, ref reference.NamedTagged) (bool, error) {
	switch iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		return icb.db.ImageExists(ctx, ref)
	case model.CustomBuild:
		// Custom build doesn't have a good way to check if the ref still exists in the image
		// store, so just assume we can.
		return true, nil
	}
	return false, fmt.Errorf("image %q has no valid buildDetails (neither "+
		"DockerBuild nor CustomBuild)", iTarget.Refs.ConfigurationRef)
}

func (icb *ImageBuilder) Build(ctx context.Context, iTarget model.ImageTarget,
	ps *build.PipelineState) (refs container.TaggedRefs, err error) {
	userFacingRefName := container.FamiliarString(iTarget.Refs.ConfigurationRef)

	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)

		refs, err = icb.db.BuildImage(ctx, ps, iTarget.Refs, bd,
			ignore.CreateBuildContextFilter(iTarget))

		if err != nil {
			return container.TaggedRefs{}, err
		}
	case model.CustomBuild:
		ps.StartPipelineStep(ctx, "Building Custom Build: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)
		refs, err = icb.custb.Build(ctx, iTarget.Refs, bd)
		if err != nil {
			return container.TaggedRefs{}, err
		}
	default:
		// Theoretically this should never trip b/c we `validate` the manifest beforehand...?
		// If we get here, something is very wrong.
		return container.TaggedRefs{}, fmt.Errorf("image %q has no valid buildDetails (neither "+
			"DockerBuild nor CustomBuild)", iTarget.Refs.ConfigurationRef)
	}

	return refs, nil
}
