package hud

import (
	"context"
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

var _ HeadsUpDisplay = &DisabledHud{}

type DisabledHud struct {
	ProcessedLogs logstore.Checkpoint
}

func NewDisabledHud() HeadsUpDisplay {
	return &DisabledHud{}
}

func (h *DisabledHud) Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error {
	return nil
}

func (h *DisabledHud) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	logs := state.LogStore.ContinuingString(h.ProcessedLogs)
	checkpoint := state.LogStore.Checkpoint()
	st.RUnlockState()

	fmt.Print(logs)
	h.ProcessedLogs = checkpoint
}
