package engine

import (
	"context"
	"fmt"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

// TODO(nick): Rename these builders. ImageBuilder should really be called DockerBuilder,
// and this struct should be called ImageBuilder.
type imageAndCacheBuilder struct {
	ib         build.ImageBuilder
	custb      build.CustomBuilder
	updateMode buildcontrol.UpdateMode
}

func NewImageAndCacheBuilder(ib build.ImageBuilder, custb build.CustomBuilder, updateMode buildcontrol.UpdateMode) *imageAndCacheBuilder {
	return &imageAndCacheBuilder{
		ib:         ib,
		custb:      custb,
		updateMode: updateMode,
	}
}

func (icb *imageAndCacheBuilder) Build(ctx context.Context, iTarget model.ImageTarget, state store.BuildState,
	ps *build.PipelineState) (refs container.TaggedRefs, err error) {
	userFacingRefName := container.FamiliarString(iTarget.Refs.ConfigurationRef)

	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)

		refs, err = icb.ib.BuildImage(ctx, ps, iTarget.Refs, bd,
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
