package store

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
)

func NewLogActionLogger(ctx context.Context, dispatch func(action Action)) logger.Logger {
	l := logger.Get(ctx)
	return logger.NewFuncLogger(l.SupportsColor(), l.Level(), func(level logger.Level, b []byte) error {
		if l.Level() >= level {
			dispatch(LogAction{
				LogEvent: LogEvent{
					Timestamp: time.Now(),
					Msg:       append([]byte{}, b...),
				},
			})
		}
		return nil
	})
}
