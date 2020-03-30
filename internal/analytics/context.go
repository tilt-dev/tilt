package analytics

import "context"

type contextKey struct{}

var analyticsContextKey = contextKey{}

func WithAnalytics(ctx context.Context, a *TiltAnalytics) context.Context {
	return context.WithValue(ctx, analyticsContextKey, a)
}

func Get(ctx context.Context) *TiltAnalytics {
	val := ctx.Value(analyticsContextKey)

	if val != nil {
		return val.(*TiltAnalytics)
	}

	// No analytics found in context, something is wrong.
	panic("Called analytics.Get(ctx) on a context with no analytics attached!")
}
