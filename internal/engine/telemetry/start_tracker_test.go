package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestStart(t *testing.T) {
	ctx := context.Background()

	fp := newFakeProcessor()

	tp := sdktrace.NewTracerProvider()
	tp.RegisterSpanProcessor(fp)
	cst := NewStartTracker(tp.Tracer("tilt.dev/test"))

	st := store.NewTestingStore()
	manifest := model.Manifest{Name: "test"}
	mt := store.NewManifestTarget(manifest)
	engineState := store.EngineState{ManifestTargets: map[model.ManifestName]*store.ManifestTarget{
		model.ManifestName("test"): mt,
	}}
	st.SetState(engineState)
	_ = cst.OnChange(ctx, st, store.LegacyChangeSummary())

	// first run span should be started
	span, exists := fp.spans["first_run"]
	require.True(t, exists)
	assert.Zero(t, span.EndTime())

	// first run span should still not be ended
	_ = cst.OnChange(ctx, st, store.LegacyChangeSummary())
	span, exists = fp.spans["first_run"]
	require.True(t, exists)
	assert.Zero(t, span.EndTime())

	engineState.CompletedBuildCount = 1
	engineState.ManifestTargets[manifest.ManifestName()].State.BuildHistory = append(engineState.ManifestTargets[manifest.ManifestName()].State.BuildHistory, model.BuildRecord{StartTime: time.Now()})
	st.SetState(engineState)
	_ = cst.OnChange(ctx, st, store.LegacyChangeSummary())

	// first run span should be ended
	span, exists = fp.spans["first_run"]
	require.True(t, exists)
	assert.NotZero(t, span.EndTime())

	_ = cst.OnChange(ctx, st, store.LegacyChangeSummary())

	// first run span should still be ended
	span, exists = fp.spans["first_run"]
	require.True(t, exists)
	assert.NotZero(t, span.EndTime())
}

type capturingProcessor struct {
	spans     map[string]sdktrace.ReadOnlySpan
	processor sdktrace.SpanProcessor
}

func (f *capturingProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	f.processor.OnStart(parent, s)
	f.spans[s.Name()] = s
}

func (f *capturingProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	f.processor.OnEnd(s)
	f.spans[s.Name()] = s
}

func (f *capturingProcessor) Shutdown(ctx context.Context) error {
	return f.processor.Shutdown(ctx)
}

func (f *capturingProcessor) ForceFlush(ctx context.Context) error {
	return f.processor.ForceFlush(ctx)
}

func newFakeProcessor() *capturingProcessor {
	return &capturingProcessor{
		spans:     make(map[string]sdktrace.ReadOnlySpan),
		processor: sdktrace.NewSimpleSpanProcessor(nil),
	}
}

var _ sdktrace.SpanProcessor = &capturingProcessor{}
