package analytics

import (
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"
)

const TagVersion = "version"
const TagOS = "os"
const TagGitRepoHash = "git.origin"

// An Analytics that allows opting in/out at runtime.
type TiltAnalytics struct {
	opter       AnalyticsOpter
	a           analytics.Analytics
	tiltVersion string

	// We make this a constant pointer to a struct.
	// That way, the struct returned by WithoutGlobalTags() can
	// point to the same opt set.
	opt *optSet
}

type optSet struct {
	env      analytics.Opt
	user     analytics.Opt
	tiltfile analytics.Opt
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
		opt: &optSet{
			env:      envOpt,
			user:     userOpt,
			tiltfile: analytics.OptDefault,
		},
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
	ta.opt.env = analytics.OptDefault
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
	if ta.EffectiveOpt() != analytics.OptOut {
		ta.a.Count(name, tags, n)
	}
}

func (ta *TiltAnalytics) Incr(name string, tags map[string]string) {
	if ta.EffectiveOpt() != analytics.OptOut {
		ta.a.Incr(name, tags)
	}
}

func (ta *TiltAnalytics) Timer(name string, dur time.Duration, tags map[string]string) {
	if ta.EffectiveOpt() != analytics.OptOut {
		ta.a.Timer(name, dur, tags)
	}
}

func (ta *TiltAnalytics) Flush(timeout time.Duration) {
	ta.a.Flush(timeout)
}

func (ta *TiltAnalytics) UserOpt() analytics.Opt {
	return ta.opt.user
}

func (ta *TiltAnalytics) TiltfileOpt() analytics.Opt {
	return ta.opt.tiltfile
}

func (ta *TiltAnalytics) EffectiveOpt() analytics.Opt {
	if ta.opt.env != analytics.OptDefault {
		return ta.opt.env
	}
	if ta.opt.tiltfile != analytics.OptDefault {
		return ta.opt.tiltfile
	}
	return ta.opt.user
}

func (ta *TiltAnalytics) SetUserOpt(opt analytics.Opt) error {
	if opt == ta.opt.user {
		return nil
	}
	ta.opt.user = opt
	return ta.opter.SetUserOpt(opt)
}

func (ta *TiltAnalytics) SetTiltfileOpt(opt analytics.Opt) {
	ta.opt.tiltfile = opt
}

func (ta *TiltAnalytics) WithoutGlobalTags() analytics.Analytics {
	return &TiltAnalytics{
		opter:       ta.opter,
		a:           ta.a.WithoutGlobalTags(),
		tiltVersion: ta.tiltVersion,
		opt:         ta.opt,
	}
}

var _ analytics.Analytics = &TiltAnalytics{}
