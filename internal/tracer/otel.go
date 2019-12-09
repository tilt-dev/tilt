package tracer

import (
	"context"

	"github.com/windmilleng/wmclient/pkg/dirs"
	"go.opentelemetry.io/otel/api/global"
	apitrace "go.opentelemetry.io/otel/api/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var tracer apitrace.Tracer

func InitOpenTelemetry(dir *dirs.WindmillDir) (Locker, error) {
	exporter, err := newExporter(dir)
	if err != nil {
		return nil, err
	}

	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}))
	if err != nil {
		return nil, err
	}
	tp.RegisterSpanProcessor(exporter)
	global.SetTraceProvider(tp)
	tracer = tp.Tracer("tilt.dev/usage") // global
	return &exporter.outgoingMu, nil
}

func Start(ctx context.Context, spanName string, opts ...apitrace.SpanOption) (context.Context, apitrace.Span) {
	return tracer.Start(ctx, spanName, opts...)
}
