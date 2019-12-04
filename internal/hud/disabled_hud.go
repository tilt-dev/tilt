package hud

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

var _ HeadsUpDisplay = &DisabledHud{}

type DisabledHud struct {
	mu sync.RWMutex

	ProcessedLogByteCount int
}

func NewDisabledHud() HeadsUpDisplay {
	return &DisabledHud{}
}

func (h *DisabledHud) Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error {
	return nil
}

func (h *DisabledHud) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	log := state.Log
	st.RUnlockState()

	h.mu.Lock()
	defer h.mu.Unlock()

	h.printLatestLogs(ctx, log)
}

// printLatestLogs prints the new logs to stdout.
func (h *DisabledHud) printLatestLogs(ctx context.Context, log model.Log) {
	logLen := log.Len()
	if h.ProcessedLogByteCount < logLen {
		fmt.Print(log.String()[h.ProcessedLogByteCount:])
	}

	h.ProcessedLogByteCount = logLen
}
