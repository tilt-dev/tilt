package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/trace"
	"google.golang.org/grpc/codes"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestStart(t *testing.T) {
	ctx := context.Background()
	ft := newFakeTracer()
	cst := NewStartTracker(ft)

	st := store.NewTestingStore()
	manifest := model.Manifest{Name: "test"}
	mt := store.NewManifestTarget(manifest)
	engineState := store.EngineState{ManifestTargets: map[model.ManifestName]*store.ManifestTarget{
		model.ManifestName("test"): mt,
	}}
	st.SetState(engineState)
	cst.OnChange(ctx, st)

	// first run span should be started
	span, exists := ft.spans["first_run"]
	require.True(t, exists)
	assert.False(t, span.ended)

	// first run span should still not be ended
	cst.OnChange(ctx, st)
	span, exists = ft.spans["first_run"]
	require.True(t, exists)
	assert.False(t, span.ended)

	engineState.CompletedBuildCount = 1
	engineState.ManifestTargets[manifest.ManifestName()].State.BuildHistory = append(engineState.ManifestTargets[manifest.ManifestName()].State.BuildHistory, model.BuildRecord{StartTime: time.Now()})
	st.SetState(engineState)
	cst.OnChange(ctx, st)

	// first run span should be ended
	span, exists = ft.spans["first_run"]
	require.True(t, exists)
	assert.True(t, span.ended)

	cst.OnChange(ctx, st)

	// first run span should still be ended
	span, exists = ft.spans["first_run"]
	require.True(t, exists)
	assert.True(t, span.ended)
}

type fakeSpanState struct {
	ended bool
}

type fakeSpan struct {
	state *fakeSpanState
}

func (s *fakeSpan) Tracer() trace.Tracer {
	return nil
}
func (s *fakeSpan) End(options ...trace.EndOption) {
	s.state.ended = true
}
func (s *fakeSpan) AddEvent(ctx context.Context, msg string, attrs ...core.KeyValue) {}
func (s *fakeSpan) AddEventWithTimestamp(ctx context.Context, timestamp time.Time, msg string, attrs ...core.KeyValue) {
}
func (s *fakeSpan) IsRecording() bool { return true }
func (s *fakeSpan) SpanContext() core.SpanContext {
	return core.SpanContext{}
}
func (s *fakeSpan) SetStatus(codes.Code)           {}
func (s *fakeSpan) SetName(name string)            {}
func (s *fakeSpan) SetAttributes(...core.KeyValue) {}

type fakeTracer struct {
	spans map[string]*fakeSpanState
}

func newFakeTracer() *fakeTracer {
	return &fakeTracer{spans: map[string]*fakeSpanState{}}
}

func (f *fakeTracer) Start(ctx context.Context, spanName string, startOpts ...trace.SpanOption) (context.Context, trace.Span) {
	spanState := &fakeSpanState{}
	f.spans[spanName] = spanState

	return ctx, &fakeSpan{spanState}
}

func (f *fakeTracer) WithSpan(
	ctx context.Context,
	spanName string,
	fn func(ctx context.Context) error,
) error {
	return nil
}
