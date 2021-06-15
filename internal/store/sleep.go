package store

import (
	"context"
	"time"
)

type Sleeper interface {
	// A cancelable Sleep(). Exits immediately if the context is canceled.
	Sleep(ctx context.Context, d time.Duration)
}

type sleeper struct{}

func (s sleeper) Sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

func DefaultSleeper() Sleeper {
	return sleeper{}
}
