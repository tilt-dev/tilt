package output

import (
	"context"
	"os"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/output"
)

// CtxForTest returns a context.Context suitable for use in tests.
// Currently, this means that it has a Logger attached.
func CtxForTest() context.Context {
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx := logger.WithLogger(context.Background(), l)
	ctx = output.WithOutputter(ctx, output.NewOutputter(l))
	return ctx
}
