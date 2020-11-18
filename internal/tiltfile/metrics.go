package tiltfile

import (
	"context"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	octag "go.opencensus.io/tag"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// Metric aggregations
var keyExecError = octag.MustNewKey("error")

var TiltfileExecDuration = stats.Float64(
	"tiltfile_exec_duration",
	"Tiltfile exec duration",
	stats.UnitMilliseconds)

var TiltfileExecDurationDistribution = view.Distribution(
	10, 100, 500, 1000, 2000, 5000,
	10000, 15000, 20000, 30000, 45000, 60000, 120000,
	240000, 480000, 1000000, 2000000, 5000000)

var TiltfileExecDurationView = &view.View{
	Name:        "tiltfile_exec_duration_dist",
	Measure:     TiltfileExecDuration,
	Aggregation: TiltfileExecDurationDistribution,
	Description: "Tiltfile exec time, by image ref",
	TagKeys:     []octag.Key{keyExecError},
}

var TiltfileExecCount = &view.View{
	Name:        "tiltfile_exec_count",
	Measure:     TiltfileExecDuration,
	Aggregation: view.Count(),
	Description: "Tiltfile exec count",
	TagKeys:     []octag.Key{keyExecError},
}

func reportTiltfileExecMetrics(ctx context.Context, loadDur time.Duration, hasError bool) {
	latencyMs := float64(loadDur / time.Millisecond)
	errorTag := "0"
	if hasError {
		errorTag = "1"
	}
	recErr := stats.RecordWithTags(ctx,
		[]octag.Mutator{octag.Upsert(keyExecError, errorTag)},
		TiltfileExecDuration.M(latencyMs))
	if recErr != nil {
		logger.Get(ctx).Debugf("imageBuilder stats: %v", recErr)
	}
}
