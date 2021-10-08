package buildcontrol

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Metric and label names must match the following rules:
// https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
var KeyImageRef = tag.MustNewKey("image_ref")

var KeyBuildError = tag.MustNewKey("build_error")

// Loosely adapted from how opencensus does HTTP aggregations:
// https://github.com/census-instrumentation/opencensus-specs/blob/master/stats/HTTP.md#http-stats
// https://pkg.go.dev/go.opencensus.io/plugin/ochttp
var ImageBuildDuration = stats.Float64(
	"image_build_duration",
	"Image build duration",
	stats.UnitMilliseconds)

var ImageBuildDurationDistribution = view.Distribution(
	10, 100, 500, 1000, 2000, 5000,
	10000, 15000, 20000, 30000, 45000, 60000, 120000,
	240000, 480000, 1000000, 2000000, 5000000)

var ImageBuildDurationView = &view.View{
	Name:        "image_build_duration_dist",
	Measure:     ImageBuildDuration,
	Aggregation: ImageBuildDurationDistribution,
	Description: "Image build time, by image ref",
	TagKeys:     []tag.Key{KeyImageRef, KeyBuildError},
}

var ImageBuildCount = &view.View{
	Name:        "image_build_count",
	Measure:     ImageBuildDuration,
	Aggregation: view.Count(),
	Description: "Image build count",
	TagKeys:     []tag.Key{KeyImageRef, KeyBuildError},
}

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
	startTime := time.Now()
	ctx, err = tag.New(ctx, tag.Upsert(KeyImageRef, userFacingRefName))
	if err != nil {
		return container.TaggedRefs{}, err
	}

	defer func() {
		latencyMs := float64(time.Since(startTime)) / float64(time.Millisecond)
		errorTag := "0"
		if err != nil {
			errorTag = "1"
		}
		recErr := stats.RecordWithTags(ctx,
			[]tag.Mutator{tag.Upsert(KeyBuildError, errorTag)},
			ImageBuildDuration.M(latencyMs))
		if recErr != nil {
			logger.Get(ctx).Debugf("ImageBuilder stats: %v", recErr)
		}
	}()

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
