package analytics

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/wmclient/pkg/analytics"
)

// An Analytics that:
// 1. Has `IncrIfUnopted` to report anonymous metrics only for users who have not opted in/out (or the choice that they
//    did opt in/out).
// 2. Ignores all other calls from users who have not opted in.
// 3. Allows opting in/out at runtime.
type TiltAnalytics struct {
	opt   analytics.Opt
	opter AnalyticsOpter
	a     analytics.Analytics
}

// An AnalyticsOpter can record a user's choice (opt-in or opt-out)
// in re: Tilt recording analytics.
type AnalyticsOpter interface {
	SetOpt(opt analytics.Opt) error
}

type NullOpter struct{}

func (NullOpter) SetOpt(opt analytics.Opt) error {
	return nil
}

var _ AnalyticsOpter = NullOpter{}

func NewTiltAnalytics(opt analytics.Opt, opter AnalyticsOpter, analytics analytics.Analytics) *TiltAnalytics {
	return &TiltAnalytics{opt, opter, analytics}
}

func NewMemoryTiltAnalytics(opter AnalyticsOpter) (*analytics.MemoryAnalytics, *TiltAnalytics) {
	ma := analytics.NewMemoryAnalytics()
	return ma, NewTiltAnalytics(analytics.OptIn, opter, ma)
}

func (ta *TiltAnalytics) Count(name string, tags map[string]string, n int) {
	if ta.opt == analytics.OptIn {
		ta.a.Count(name, tags, n)
	}
}

func (ta *TiltAnalytics) Incr(name string, tags map[string]string) {
	if ta.opt == analytics.OptIn {
		ta.a.Incr(name, tags)
	}
}

func (ta *TiltAnalytics) IncrIfUnopted(name string) {
	if ta.opt == analytics.OptDefault {
		ta.a.IncrAnonymous(name, map[string]string{})
	}
}

func (ta *TiltAnalytics) IncrAnonymous(name string, tags map[string]string) {
	// Q: This is confusing! Isn't IncrAnonymous only for OptDefault?
	// A: ...well, that was why it was added. If you drop a random IncrAnonymous call somewhere else in the code
	//    and nothing happens, would you be surprised? We could eliminate IncrIfUnopted and make IncrAnonymous
	//    only run when opt == OptDefault, but then there's it feels weird that some methods only work if OptIn and
	//    some only work if OptDefault, and it's not really clear from the names why.
	//    By making IncrIfUnopted its own method, we go with the mental model of "IncrIfUnopted is explicit about its
	//    relationship to opt, and all other methods only run on OptIn"
	if ta.opt == analytics.OptIn {
		ta.a.IncrAnonymous(name, tags)
	}
}

func (ta *TiltAnalytics) Timer(name string, dur time.Duration, tags map[string]string) {
	if ta.opt == analytics.OptIn {
		ta.a.Timer(name, dur, tags)
	}
}

func (ta *TiltAnalytics) Flush(timeout time.Duration) {
	ta.a.Flush(timeout)
}

func (ta *TiltAnalytics) SetOpt(opt analytics.Opt) error {
	if opt == analytics.OptIn {
		ta.IncrIfUnopted("analytics.opt.in")
	}
	// no logging on opt-out, because, well, opting out means the user just told us not to report data on them!
	ta.opt = opt
	return ta.opter.SetOpt(ta.opt)
}

var _ analytics.Analytics = &TiltAnalytics{}

func (ta *TiltAnalytics) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()
	if state.AnalyticsOpt != ta.opt {
		err := ta.SetOpt(state.AnalyticsOpt)
		if err != nil {
			logger.Get(ctx).Infof("error saving analytics opt-in status")
		}
	}
}

var _ store.Subscriber = &TiltAnalytics{}
