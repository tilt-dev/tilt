package testutils

import (
	"context"

	"github.com/windmilleng/tilt/internal/logger"
)

// CtxForTest returns a context.Context suitable for use in tests.
// Currently, this means that it has a Logger attached.
func CtxForTest() context.Context {
	return logger.WithLogger(context.Background(), logger.NewLogger(logger.DebugLvl))
}
