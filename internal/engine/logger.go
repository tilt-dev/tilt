package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
)

func NewLogActionLogger(ctx context.Context, dispatch func(action store.Action)) logger.Logger {
	l := logger.Get(ctx)
	return logger.NewFuncLogger(l.SupportsColor(), l.Level(), func(level logger.Level, b []byte) error {
		if l.Level() >= level {
			dispatch(LogAction{append([]byte{}, b...)})
		}
		return nil
	})
}
