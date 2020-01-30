package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/github"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestSleepAfterFailure(t *testing.T) {
	f := newVersionCheckerFixture(t)

	errorMsg := "github error!"

	f.delay = infiniteDelay
	f.ghc.LatestReleaseErr = errors.New(errorMsg)

	f.vc.OnChange(f.ctx, f.store)

	f.waitUntil(t, func(f *versionCheckerFixture) bool {
		return strings.Contains(f.out.String(), errorMsg)
	})

	// Hackily hope a sleep triggers the bug we want to test (immediate retry on error, leading to infinite
	// retries). This at least currently repro'd the bug 100/100 times. Unclear if there's a less hacky option.
	time.Sleep(5 * time.Millisecond)

	f.cancel()

	assert.Equal(t, 1, strings.Count(f.out.String(), errorMsg))
}

func TestVersionCheckDisabledForDev(t *testing.T) {
	f := newVersionCheckerFixture(t)

	state := f.store.LockMutableStateForTesting()
	state.TiltBuildInfo = model.TiltBuild{Dev: true}
	f.store.UnlockMutableState()

	f.vc.OnChange(f.ctx, f.store)

	require.False(t, f.vc.started)
}

type versionCheckerFixture struct {
	ctx    context.Context
	store  *store.Store
	vc     *TiltVersionChecker
	cancel context.CancelFunc
	delay  time.Duration
	ghc    *github.FakeClient
	out    *bufsync.ThreadSafeBuffer
}

const infiniteDelay = -1

func newVersionCheckerFixture(t *testing.T) *versionCheckerFixture {
	out := bufsync.NewThreadSafeBuffer()
	reducer := func(ctx context.Context, state *store.EngineState, action store.Action) {
		event, ok := action.(store.LogAction)
		if !ok {
			t.Errorf("Expected action type LogAction. Actual: %T", action)
		}
		_, err := out.Write(event.Message())
		require.NoError(t, err)
	}

	st := store.NewStore(store.Reducer(reducer), store.LogActionsFlag(false))

	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = logger.WithLogger(ctx, l)
	go func() {
		err := st.Loop(ctx)
		testutils.FailOnNonCanceledErr(t, err, "store.Loop failed")
	}()

	ret := &versionCheckerFixture{
		ctx:    ctx,
		store:  st,
		cancel: cancel,
		out:    out,
	}

	tiltVersionCheckTimerMaker := func(d time.Duration) <-chan time.Time {
		if ret.delay == infiniteDelay {
			return nil
		} else {
			return time.After(ret.delay)
		}

	}
	ghc := &github.FakeClient{}
	vc := NewTiltVersionChecker(func() github.Client { return ghc }, tiltVersionCheckTimerMaker)

	ret.vc = vc
	ret.ghc = ghc
	return ret
}

func (vcf *versionCheckerFixture) waitUntil(t *testing.T, pred func(vcf *versionCheckerFixture) bool) {
	start := time.Now()
	for time.Since(start) < time.Second {
		if pred(vcf) {
			return
		}
	}
	t.Fatal("timed out waiting for predicate to be true")
}
