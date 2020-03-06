package cloud

import (
	"context"
	"io"
	"os"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

func WriteSnapshot(ctx context.Context, store *store.Store, path string) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		logger.Get(ctx).Errorf("Writing snapshot to file: %v", err)
	}

	state := store.RLockState()
	defer store.RUnlockState()

	err = WriteSnapshotTo(ctx, state, f)
	if err != nil {
		logger.Get(ctx).Errorf("Writing snapshot to file: %v", err)
	}
}

func WriteSnapshotTo(ctx context.Context, state store.EngineState, w io.Writer) error {
	snapshot, err := ToSnapshot(state)
	if err != nil {
		return err
	}

	jsEncoder := &runtime.JSONPb{
		OrigName: false,
		Indent:   "  ",
	}
	return jsEncoder.NewEncoder(w).Encode(snapshot)
}
