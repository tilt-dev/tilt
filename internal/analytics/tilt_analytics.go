package analytics

import (
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"
)

const TagVersion = "version"
const TagOS = "os"
const TagGitRepoHash = "git.origin"

// An Analytics that:
// 1. Has `IncrIfUnopted` to report anonymous metrics only for users who have not opted in/out (or the choice that they
//    did opt in/out).
// 2. Ignores all other calls from users who have not opted in.
// 3. Allows opting in/out at runtime.
type TiltAnalytics struct {
	opter       AnalyticsOpter
	a           analytics.Analytics
	tiltVersion string

	envOpt      analytics.Opt
	userOpt     analytics.Opt
	tiltfileOpt analytics.Opt
}

// An AnalyticsOpter can record a user's choice (opt-in or opt-out)
// in re: Tilt recording analytics.
type AnalyticsOpter interface {
	SetUserOpt(opt analytics.Opt) error
	ReadUserOpt() (analytics.Opt, error)
}

func NewTiltAnalytics(opter AnalyticsOpter, a analytics.Analytics, tiltVersion string) (*TiltAnalytics, error) {
	userOpt, err := opter.ReadUserOpt()
	if err != nil {
		return nil, err
	}
	envOpt := analytics.OptDefault
	if ok, _ := IsAnalyticsDisabledFromEnv(); ok {
		envOpt = analytics.OptOut
	}
	return &TiltAnalytics{
		opter:       opter,
		a:           a,
		tiltVersion: tiltVersion,
		envOpt:      envOpt,
		userOpt:     userOpt,
	}, nil
}

// NOTE: if you need a ctx as well, use testutils.CtxAndAnalyticsForTest so that you get
// a ctx with the correct analytics baked in.
func NewMemoryTiltAnalyticsForTest(opter AnalyticsOpter) (*analytics.MemoryAnalytics, *TiltAnalytics) {
	ma := analytics.NewMemoryAnalytics()
	ta, err := NewTiltAnalytics(opter, ma, "v0.0.0")
	if err != nil {
		panic(err)
	}
	ta.envOpt = analytics.OptDefault
	return ma, ta
}

func (ta *TiltAnalytics) GlobalTag(name string) (string, bool) {
	return ta.a.GlobalTag(name)
}
func (ta *TiltAnalytics) MachineHash() string {
	id, _ := ta.GlobalTag(analytics.TagMachine)
	return id
}
func (ta *TiltAnalytics) GitRepoHash() string {
	id, _ := ta.GlobalTag(TagGitRepoHash)
	return id
}
func (ta *TiltAnalytics) Count(name string, tags map[string]string, n int) {
	if ta.EffectiveOpt() == analytics.OptIn {
		ta.a.Count(name, tags, n)
	}
}

func (ta *TiltAnalytics) Incr(name string, tags map[string]string) {
	if ta.EffectiveOpt() == analytics.OptIn {
		ta.a.Incr(name, tags)
	}
}

func (ta *TiltAnalytics) IncrIfUnopted(name string) {
	if ta.EffectiveOpt() == analytics.OptDefault {
		ta.a.IncrAnonymous(name, map[string]string{"version": ta.tiltVersion})
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
	if ta.EffectiveOpt() == analytics.OptIn {
		ta.a.IncrAnonymous(name, tags)
	}
}

func (ta *TiltAnalytics) Timer(name string, dur time.Duration, tags map[string]string) {
	if ta.EffectiveOpt() == analytics.OptIn {
		ta.a.Timer(name, dur, tags)
	}
}

func (ta *TiltAnalytics) Flush(timeout time.Duration) {
	ta.a.Flush(timeout)
}

func (ta *TiltAnalytics) UserOpt() analytics.Opt {
	return ta.userOpt
}

func (ta *TiltAnalytics) TiltfileOpt() analytics.Opt {
	return ta.tiltfileOpt
}

func (ta *TiltAnalytics) EffectiveOpt() analytics.Opt {
	if ta.envOpt != analytics.OptDefault {
		return ta.envOpt
	}
	if ta.tiltfileOpt != analytics.OptDefault {
		return ta.tiltfileOpt
	}
	return ta.userOpt
}

func (ta *TiltAnalytics) SetUserOpt(opt analytics.Opt) error {
	if opt == ta.userOpt {
		return nil
	}
	ta.userOpt = opt
	return ta.opter.SetUserOpt(opt)
}

func (ta *TiltAnalytics) SetTiltfileOpt(opt analytics.Opt) {
	ta.tiltfileOpt = opt
}

var _ analytics.Analytics = &TiltAnalytics{}
