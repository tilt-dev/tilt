package cli

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/pkg/logger"

	giturls "github.com/whilp/git-urls"

	"github.com/windmilleng/wmclient/pkg/analytics"
)

const tiltAppName = "tilt"
const analyticsURLEnvVar = "TILT_ANALYTICS_URL"

// Testing analytics locally:
// (after `npm install http-echo-server -g`)
// In one window: `PORT=9988 http-echo-server`
// In another: `TILT_ANALYTICS_URL=http://localhost:9988 tilt up`
// Analytics requests will show up in the http-echo-server window.

type analyticsOpter struct{}

var _ tiltanalytics.AnalyticsOpter = analyticsOpter{}

func (ao analyticsOpter) ReadUserOpt() (analytics.Opt, error) {
	return analytics.OptStatus()
}

func (ao analyticsOpter) SetUserOpt(opt analytics.Opt) error {
	return analytics.SetOpt(opt)
}

type analyticsLogger struct {
	logger logger.Logger
}

func (al analyticsLogger) Printf(fmt string, v ...interface{}) {
	al.logger.Debugf(fmt, v...)
}

func newAnalytics(l logger.Logger) (*tiltanalytics.TiltAnalytics, error) {
	var err error

	options := []analytics.Option{}
	// enabled: true because TiltAnalytics wraps the RemoteAnalytics and has its own guards for whether analytics
	//   is enabled. When TiltAnalytics decides to pass a call through to RemoteAnalytics, it should always work.
	options = append(options,
		analytics.WithGlobalTags(globalTags()),
		analytics.WithEnabled(true),
		analytics.WithLogger(analyticsLogger{logger: l}))
	analyticsURL := os.Getenv(analyticsURLEnvVar)
	if analyticsURL != "" {
		options = append(options, analytics.WithReportURL(analyticsURL))
	}
	backingAnalytics, err := analytics.NewRemoteAnalytics(tiltAppName, options...)
	if err != nil {
		return nil, err
	}

	return tiltanalytics.NewTiltAnalytics(analyticsOpter{}, backingAnalytics, provideTiltInfo().AnalyticsVersion())
}

func globalTags() map[string]string {
	ret := map[string]string{
		tiltanalytics.TagVersion: provideTiltInfo().AnalyticsVersion(),
		tiltanalytics.TagOS:      runtime.GOOS,
	}

	// store a hash of the git remote to help us guess how many users are running it on the same repository
	origin := normalizeGitRemote(gitOrigin("."))
	if origin != "" {
		ret[tiltanalytics.TagGitRepoHash] = tiltanalytics.HashMD5(origin)
	}

	return ret
}

func gitOrigin(fromDir string) string {
	cmd := exec.Command("git", "-C", fromDir, "remote", "get-url", "origin")
	b, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimRight(string(b), "\n")
}

func normalizeGitRemote(s string) string {
	u, err := giturls.Parse(string(s))
	if err != nil {
		return s
	}

	// treat "http://", "https://", "git://", "ssh://", etc as equiv
	u.Scheme = ""

	u.User = nil

	// github.com/windmilleng/tilt is the same as github.com/windmilleng/tilt/
	if strings.HasSuffix(u.Path, "/") {
		u.Path = u.Path[:len(u.Path)-1]
	}

	// github.com/windmilleng/tilt is the same as github.com/windmilleng/tilt.git
	if strings.HasSuffix(u.Path, ".git") {
		u.Path = u.Path[:len(u.Path)-4]
	}

	return u.String()
}
