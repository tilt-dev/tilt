package hud

import (
	"context"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type TerminalStream struct {
	ProcessedLogs logstore.Checkpoint
	printer       *IncrementalPrinter
	filter        LogFilter
	store         store.RStore
}

func NewTerminalStream(printer *IncrementalPrinter, filter LogFilter, store store.RStore) *TerminalStream {
	return &TerminalStream{
		printer: printer,
		filter:  filter,
		store:   store,
	}
}

// TODO(nick): We should change this API so that TearDown gets
// the RStore one last time.
func (h *TerminalStream) TearDown(ctx context.Context) {
	if !h.isEnabled(h.store) {
		return
	}

	_ = h.OnChange(ctx, h.store, store.LegacyChangeSummary())

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

func (h *TerminalStream) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) error {
	if !h.isEnabled(st) {
		return nil
	}

	state := st.RLockState()
	lines := state.LogStore.ContinuingLinesWithOptions(h.ProcessedLogs, logstore.LineOptions{
		SuppressPrefix: h.filter.SuppressPrefix(),
	})
	lines = h.filter.Apply(lines)

	checkpoint := state.LogStore.Checkpoint()
	st.RUnlockState()

	h.printer.Print(lines)
	h.ProcessedLogs = checkpoint
	return nil
}

var _ store.TearDowner = &TerminalStream{}
