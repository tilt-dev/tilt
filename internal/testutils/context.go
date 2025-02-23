package testutils

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/tilt-dev/wmclient/pkg/analytics"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/pkg/logger"
)

func LoggerCtx() context.Context {
	return logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stdout))
}

// CtxAndAnalyticsForTest returns a context.Context suitable for use in tests (i.e. with
// logger & analytics attached), and the analytics it contains.
func CtxAndAnalyticsForTest() (context.Context, *analytics.MemoryAnalytics, *tiltanalytics.TiltAnalytics) {
	ctx := LoggerCtx()

	opter := tiltanalytics.NewFakeOpter(analytics.OptIn)
	ma, ta := tiltanalytics.NewMemoryTiltAnalyticsForTest(opter)
	ctx = tiltanalytics.WithAnalytics(ctx, ta)

	return ctx, ma, ta
}

func ForkedCtxAndAnalyticsWithOpterForTest(w io.Writer, o tiltanalytics.AnalyticsOpter) (context.Context, *analytics.MemoryAnalytics, *tiltanalytics.TiltAnalytics) {
	ctx := LoggerCtx()
	ctx = logger.CtxWithForkedOutput(ctx, w)

	ma, ta := tiltanalytics.NewMemoryTiltAnalyticsForTest(o)
	ctx = tiltanalytics.WithAnalytics(ctx, ta)

	return ctx, ma, ta
}

// CtxForTest returns a context.Context suitable for use in tests (i.e. with
// logger attached), and with all output being copied to `w`
func ForkedCtxAndAnalyticsForTest(w io.Writer) (context.Context, *analytics.MemoryAnalytics, *tiltanalytics.TiltAnalytics) {
	opter := tiltanalytics.NewFakeOpter(analytics.OptIn)
	return ForkedCtxAndAnalyticsWithOpterForTest(w, opter)
}

func FailOnNonCanceledErr(t testing.TB, err error, message string) {
	if err != nil && err != context.Canceled {
		fmt.Printf("%s: %v\n", message, err)
		t.Error(err)
	}
}
