package cli

import (
	"os"
	"runtime"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/git"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/wmclient/pkg/analytics"
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

func newAnalytics(l logger.Logger, cmdName model.TiltSubcommand, tiltBuild model.TiltBuild,
	gitRemote git.GitRemote) (*tiltanalytics.TiltAnalytics, error) {
	var err error

	options := []analytics.Option{}
	// enabled: true because TiltAnalytics wraps the RemoteAnalytics and has its own guards for whether analytics
	//   is enabled. When TiltAnalytics decides to pass a call through to RemoteAnalytics, it should always work.
	options = append(options,
		analytics.WithGlobalTags(globalTags(cmdName, tiltBuild, gitRemote)),
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

	return tiltanalytics.NewTiltAnalytics(analyticsOpter{}, backingAnalytics, tiltBuild.AnalyticsVersion())
}

func globalTags(cmdName model.TiltSubcommand, tiltBuild model.TiltBuild, gr git.GitRemote) map[string]string {
	ret := map[string]string{
		tiltanalytics.TagVersion:    tiltBuild.AnalyticsVersion(),
		tiltanalytics.TagOS:         runtime.GOOS,
		tiltanalytics.TagSubcommand: cmdName.String(),
	}

	// store a hash of the git remote to help us guess how many users are running it on the same repository
	origin := gr.String()
	if origin != "" {
		ret[tiltanalytics.TagGitRepoHash] = tiltanalytics.HashMD5(origin)
	}

	return ret
}
