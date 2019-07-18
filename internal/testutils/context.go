package testutils

import (
	"context"
	"io"
	"os"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/logger"
)

// CtxForTest returns a context.Context suitable for use in tests (i.e. with
// logger & analytics attached).
func CtxForTest() context.Context {
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx := logger.WithLogger(context.Background(), l)

	_, a := analytics.NewMemoryTiltAnalyticsForTest(analytics.NullOpter{})
	ctx = analytics.WithAnalytics(ctx, a)

	return ctx
}

// CtxForTest returns a context.Context suitable for use in tests (i.e. with
// logger attached), and with all output being copied to `w`
func ForkedCtxForTest(w io.Writer) context.Context {
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx := logger.WithLogger(context.Background(), l)
	ctx = logger.CtxWithForkedOutput(ctx, w)
	return ctx
}
