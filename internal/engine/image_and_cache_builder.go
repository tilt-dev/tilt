package engine

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockerfile"
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

func (icb *imageAndCacheBuilder) Build(ctx context.Context, iTarget model.ImageTarget, state store.BuildState, ps *build.PipelineState) (reference.NamedTagged, error) {
	var n reference.NamedTagged

	userFacingRefName := container.FamiliarString(iTarget.ConfigurationRef)
	refToBuild := iTarget.DeploymentRef

	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)

		df := dockerfile.Dockerfile(bd.Dockerfile)
		ref, err := icb.ib.BuildImage(ctx, ps, refToBuild, df, bd.BuildPath,
			ignore.CreateBuildContextFilter(iTarget), bd.BuildArgs, bd.TargetStage)

		if err != nil {
			return nil, err
		}
		n = ref
	case model.CustomBuild:
		ps.StartPipelineStep(ctx, "Building Custom Build: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)
		ref, err := icb.custb.Build(ctx, refToBuild, bd)
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
