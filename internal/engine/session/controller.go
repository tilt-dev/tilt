package session

import (
	"context"

	"github.com/tilt-dev/tilt/internal/controllers/core/session"
	"github.com/tilt-dev/tilt/internal/store"
)

// A stub controller that simply schedules Session reconciliation whenever
// the engine state changes.
type Controller struct {
	r *session.Reconciler
}

var _ store.Subscriber = &Controller{}

func NewController(r *session.Reconciler) *Controller {
	return &Controller{
		r: r,
	}
}

func (c *Controller) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}
	c.r.Requeue()
	return nil
}
