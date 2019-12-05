package hud

import (
	"context"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/hud/view"
)

var _ HeadsUpDisplay = (*FakeHud)(nil)

type FakeHud struct {
	LastView  view.View
	viewState view.ViewState
	updates   chan view.View
	Canceled  bool
	Closed    bool
	closeCh   chan interface{}
}

func NewFakeHud() *FakeHud {
	return &FakeHud{
		updates: make(chan view.View, 10),
		closeCh: make(chan interface{}),
	}
}

func (h *FakeHud) Run(ctx context.Context, dispatch func(action store.Action), refreshInterval time.Duration) error {
	select {
	case <-ctx.Done():
	case <-h.closeCh:
	}
	h.Canceled = true
	return ctx.Err()
}

func (h *FakeHud) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	view := store.StateToView(state, st.StateMutex())
	st.RUnlockState()

	err := h.update(view, h.viewState)
	if err != nil {
		logger.Get(ctx).Infof("Error updating HUD: %v", err)
	}
}

func (h *FakeHud) close() {
	h.Closed = true
	close(h.closeCh)
}

func (h *FakeHud) update(v view.View, vs view.ViewState) error {
	h.LastView = v
	h.updates <- v
	return nil
}

func (h *FakeHud) WaitUntilResource(t testing.TB, ctx context.Context, msg string, name model.ManifestName, isDone func(view.Resource) bool) {
	h.WaitUntil(t, ctx, msg, func(view view.View) bool {
		res, ok := view.Resource(name)
		if !ok {
			return false
		}
		return isDone(res)
	})
}

func (h *FakeHud) WaitUntil(t testing.TB, ctx context.Context, msg string, isDone func(view.View) bool) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for: %s", msg)
		case view := <-h.updates:
			done := isDone(view)
			if done {
				return
			}
		}
	}
}
