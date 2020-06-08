package hud

import (
	"context"
	"time"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

var _ HeadsUpDisplay = &DisabledHud{}

type DisabledHud struct {
	ProcessedLogs logstore.Checkpoint
	printer       *IncrementalPrinter
	store         store.RStore
}

func NewDisabledHud(printer *IncrementalPrinter, store store.RStore) HeadsUpDisplay {
	return &DisabledHud{printer: printer, store: store}
}

func (h *DisabledHud) Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error {
	return nil
}

// TODO(nick): We should change this API so that TearDown gets
// the RStore one last time.
func (h *DisabledHud) TearDown(ctx context.Context) {
	h.OnChange(ctx, h.store)

	state := h.store.RLockState()
	uncompleted := state.LogStore.IsLastSegmentUncompleted()
	h.store.RUnlockState()

	if uncompleted {
		h.printer.PrintNewline()
	}
}

func (h *DisabledHud) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	lines := state.LogStore.ContinuingLines(h.ProcessedLogs)
	checkpoint := state.LogStore.Checkpoint()
	st.RUnlockState()

	h.printer.Print(lines)
	h.ProcessedLogs = checkpoint
}

var _ store.TearDowner = &DisabledHud{}
