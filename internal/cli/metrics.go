package cli

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/engine/metrics"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/model"
)

var exporter *metrics.DeferredExporter
var meter view.Meter

// Metric and label names must match the following rules:
// https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
var KeySubCommand = tag.MustNewKey("subcommand")

var CommandCountMeasure = stats.Int64(
	"cli_count_m",
	"Number of CLI invocations",
	stats.UnitDimensionless)

var CommandCount = &view.View{
	Name:        "cli_count",
	Measure:     CommandCountMeasure,
	Description: "Number of CLI invocations",
	TagKeys:     []tag.Key{KeySubCommand},
	Aggregation: view.Count(),
}

func initMetrics(ctx context.Context, cmdName model.TiltSubcommand) (context.Context, func() error, error) {
	exporter = metrics.NewDeferredExporter()
	view.RegisterExporter(exporter)

	// TODO(nick): This isn't quite right. Opencensus defaults are really intended
	// for in-cluster server monitoring, not commandline tools. So we need some
	// sort of Flush() mechanism to flush the whole reporting pipeline, not just
	// the exporter.
	cleanup := func() error {
		exporter.Flush()
		return exporter.Stop()
	}

	err := view.Register(
		CommandCount,
		buildcontrol.ImageBuildDurationView,
		buildcontrol.ImageBuildCount,
		buildcontrol.K8sDeployDurationView,
		buildcontrol.K8sDeployCount,
		buildcontrol.K8sDeployObjectsCount,
		tiltfile.TiltfileExecDurationView,
		tiltfile.TiltfileExecCount)
	if err != nil {
		return nil, cleanup, err
	}

	// In opencensus, we propagate tags with context rather than having
	// global tags.
	// https://github.com/census-instrumentation/opencensus-go/issues/786
	ctx, err = tag.New(ctx,
		tag.Upsert(KeySubCommand, string(cmdName)))
	return ctx, cleanup, err
}

func ProvideDeferredExporter() *metrics.DeferredExporter {
	return exporter
}

func ProvideMeter() view.Meter {
	return meter
}
