package tracer

import (
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "tilt.dev/usage"

func InitOpenTelemetry(exporter sdktrace.SpanExporter) trace.Tracer {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	sp := sdktrace.NewBatchSpanProcessor(exporter)
	tp.RegisterSpanProcessor(sp)
	tracer := tp.Tracer(tracerName)
	return tracer
}
