package tracer

import (
	"context"

	apitrace "go.opentelemetry.io/otel/api/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var tracer apitrace.Tracer

func InitOpenTelemetry(ctx context.Context, exporter sdktrace.SpanProcessor) (apitrace.Tracer, error) {
	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}))
	if err != nil {
		return nil, err
	}
	if exporter != nil {
		tp.RegisterSpanProcessor(exporter)
	}
	tracer = tp.Tracer("tilt.dev/usage")
	return tracer, nil
}
