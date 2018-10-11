package output

import (
	"context"
	"io"
	"os"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/output"
)

// CtxForTest returns a context.Context suitable for use in tests (i.e. with
// logger, outputter, etc. attached).
func CtxForTest() context.Context {
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx := logger.WithLogger(context.Background(), l)
	ctx = output.WithOutputter(ctx, output.NewOutputter(l))
	return ctx
}

// CtxForTest returns a context.Context suitable for use in tests (i.e. with
// logger, outputter, etc. attached), and with all output being copied to `w`
func ForkedCtxForTest(w io.Writer) context.Context {
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx := logger.WithLogger(context.Background(), l)
	ctx = output.WithOutputter(ctx, output.NewOutputter(l))
	ctx = output.CtxWithForkedOutput(ctx, w)
	return ctx
}
