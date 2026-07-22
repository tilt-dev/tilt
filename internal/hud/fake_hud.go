package hud

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/hud/view"
)

var _ HeadsUpDisplay = (*FakeHud)(nil)

type FakeHud struct {
	mu             sync.Mutex
	lastView       view.View
	lastViewUpdate chan struct{}

	Canceled bool
}

func NewFakeHud() *FakeHud {
	return &FakeHud{
		lastViewUpdate: make(chan struct{}),
	}
}

func (h *FakeHud) Run(ctx context.Context, dispatch func(action store.Action), refreshInterval time.Duration) error {
	select {
	case <-ctx.Done():
	}
	h.Canceled = true
	close(h.lastViewUpdate)
	return ctx.Err()
}

func (h *FakeHud) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) error {
	state := st.RLockState()
	view := StateToTerminalView(state, st.StateMutex())
	st.RUnlockState()

	h.update(view)
	return nil
}

// LastView returns the most recent view the HUD has rendered.
func (h *FakeHud) LastView() view.View {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastView
}

func (h *FakeHud) update(v view.View) {
	h.mu.Lock()
	h.lastView = v
	lastViewUpdate := h.lastViewUpdate
	h.lastViewUpdate = make(chan struct{})
	h.mu.Unlock()

	close(lastViewUpdate)
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
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	for {
		h.mu.Lock()
		lastView := h.lastView
		lastViewUpdate := h.lastViewUpdate
		h.mu.Unlock()

		if isDone(lastView) {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for: %s", msg)
		case <-lastViewUpdate:
		}
	}
}
