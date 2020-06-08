package engine

import (
	"context"
	"fmt"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/pkg/model"
)

type imageBuilder struct {
	db         build.DockerBuilder
	custb      build.CustomBuilder
	updateMode buildcontrol.UpdateMode
}

func NewImageBuilder(db build.DockerBuilder, custb build.CustomBuilder, updateMode buildcontrol.UpdateMode) *imageBuilder {
	return &imageBuilder{
		db:         db,
		custb:      custb,
		updateMode: updateMode,
	}
}

func (icb *imageBuilder) Build(ctx context.Context, iTarget model.ImageTarget,
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
