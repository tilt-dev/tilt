package store

import (
	"context"

	"github.com/windmilleng/tilt/pkg/logger"
)

func NewLogActionLogger(ctx context.Context, dispatch func(action Action)) logger.Logger {
	l := logger.Get(ctx)
	return logger.NewFuncLogger(l.SupportsColor(), l.Level(), func(level logger.Level, fields logger.Fields, b []byte) error {
		dispatch(NewGlobalLogAction(level, b))
		return nil
	})
}
