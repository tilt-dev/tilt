package hud

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

var _ HeadsUpDisplay = &DisabledHud{}

type DisabledHud struct {
	ProcessedLogs logstore.Checkpoint
	printer       *IncrementalPrinter
}

func NewDisabledHud(printer *IncrementalPrinter) HeadsUpDisplay {
	return &DisabledHud{printer: printer}
}

func (h *DisabledHud) Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error {
	return nil
}

func (h *DisabledHud) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	lines := state.LogStore.ContinuingLines(h.ProcessedLogs)
	checkpoint := state.LogStore.Checkpoint()
	st.RUnlockState()

	h.printer.Print(lines)
	h.ProcessedLogs = checkpoint
}
