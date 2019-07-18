package testutils

import (
	"context"
	"io"
	"os"

	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/logger"
)

// CtxAndAnalyticsForTest returns a context.Context suitable for use in tests (i.e. with
// logger & analytics attached), and the analytics it contains.
func CtxAndAnalyticsForTest() (context.Context, *analytics.MemoryAnalytics, *tiltanalytics.TiltAnalytics) {
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx := logger.WithLogger(context.Background(), l)

	ma, ta := tiltanalytics.NewMemoryTiltAnalyticsForTest(tiltanalytics.NullOpter{})
	ctx = tiltanalytics.WithAnalytics(ctx, ta)

	return ctx, ma, ta
}

func ForkedCtxAndAnalyticsWithOpterForTest(w io.Writer, o tiltanalytics.AnalyticsOpter) (context.Context, *analytics.MemoryAnalytics, *tiltanalytics.TiltAnalytics) {
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx := logger.WithLogger(context.Background(), l)
	ctx = logger.CtxWithForkedOutput(ctx, w)

	ma, ta := tiltanalytics.NewMemoryTiltAnalyticsForTest(o)
	ctx = tiltanalytics.WithAnalytics(ctx, ta)

	return ctx, ma, ta
}

// CtxForTest returns a context.Context suitable for use in tests (i.e. with
// logger attached), and with all output being copied to `w`
func ForkedCtxAndAnalyticsForTest(w io.Writer) (context.Context, *analytics.MemoryAnalytics, *tiltanalytics.TiltAnalytics) {
	return ForkedCtxAndAnalyticsWithOpterForTest(w, tiltanalytics.NullOpter{})
}
