package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/api/trace"

	"github.com/windmilleng/tilt/internal/store"
)

type StartTracker struct {
	tracer            trace.Tracer
	span              trace.Span
	coldStartFinished bool
}

func NewStartTracker(tracer trace.Tracer) *StartTracker {
	return &StartTracker{tracer: tracer, coldStartFinished: false}
}

func (c *StartTracker) OnChange(ctx context.Context, st store.RStore) {
	if c.coldStartFinished {
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()

	if !state.InitialBuildsCompleted() && c.span == nil {
		_, span := c.tracer.Start(ctx, "first_run")
		c.span = span
	}

	if state.InitialBuildsCompleted() && c.span != nil {
		c.span.End()
		c.coldStartFinished = true
	}
}
