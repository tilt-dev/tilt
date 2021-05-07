package uisession

import (
	"context"

	"github.com/tilt-dev/tilt/internal/store"
)

type Subscriber struct {
}

func NewSubscriber() *Subscriber {
	return &Subscriber{}
}

func (s *Subscriber) OnChange(ctx context.Context, store store.RStore, summary store.ChangeSummary) {
}

var _ store.Subscriber = &Subscriber{}
