package store

import (
	"context"

	"github.com/windmilleng/tilt/pkg/logger"
)

func NewLogActionLogger(ctx context.Context, dispatch func(action Action)) logger.Logger {
	l := logger.Get(ctx)
	return logger.NewFuncLogger(l.SupportsColor(), l.Level(), func(level logger.Level, b []byte) error {
		if l.Level() >= level {
			dispatch(NewGlobalLogEvent(level, b))
		}
		return nil
	})
}
