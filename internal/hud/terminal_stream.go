package hud

import (
	"context"
	"time"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type TerminalStream struct {
	ProcessedLogs logstore.Checkpoint
	printer       *IncrementalPrinter
	store         store.RStore
}

func NewTerminalStream(printer *IncrementalPrinter, store store.RStore) *TerminalStream {
	return &TerminalStream{printer: printer, store: store}
}

func (h *TerminalStream) Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error {
	return nil
}

// TODO(nick): We should change this API so that TearDown gets
// the RStore one last time.
func (h *TerminalStream) TearDown(ctx context.Context) {
	if !h.isEnabled(h.store) {
		return
	}

	h.OnChange(ctx, h.store)

	state := h.store.RLockState()
	uncompleted := state.LogStore.IsLastSegmentUncompleted()
	h.store.RUnlockState()

	if uncompleted {
		h.printer.PrintNewline()
	}
}

func (h *TerminalStream) isEnabled(st store.RStore) bool {
	state := st.RLockState()
	defer st.RUnlockState()
	return state.TerminalMode == store.TerminalModeStream
}

func (h *TerminalStream) OnChange(ctx context.Context, st store.RStore) {
	if !h.isEnabled(st) {
		return
	}

	state := st.RLockState()
	lines := state.LogStore.ContinuingLines(h.ProcessedLogs)
	checkpoint := state.LogStore.Checkpoint()
	st.RUnlockState()

	h.printer.Print(lines)
	h.ProcessedLogs = checkpoint
}

var _ store.TearDowner = &TerminalStream{}
