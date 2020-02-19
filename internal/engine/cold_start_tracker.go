package engine

import (
	"context"

	"go.opentelemetry.io/otel/api/trace"

	"github.com/windmilleng/tilt/internal/store"
)

type ColdStartTracker struct {
	tracer            trace.Tracer
	span              trace.Span
	coldStartFinished bool
}

func NewColdStartTracker(tracer trace.Tracer) *ColdStartTracker {
	return &ColdStartTracker{tracer: tracer, coldStartFinished: false}
}

func (c *ColdStartTracker) OnChange(ctx context.Context, st store.RStore) {
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
