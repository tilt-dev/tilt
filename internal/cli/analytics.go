package cli

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"

	"github.com/spf13/cobra"
	giturls "github.com/whilp/git-urls"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

const tiltAppName = "tilt"
const disableAnalyticsEnvVar = "TILT_DISABLE_ANALYTICS"
const analyticsURLEnvVar = "TILT_ANALYTICS_URL"

var analyticsService *tiltanalytics.TiltAnalytics
var analyticsOpt = analytics.OptDefault

// Testing analytics locally:
// (after `npm install http-echo-server -g`)
// In one window: `PORT=9988 http-echo-server`
// In another: `TILT_ANALYTICS_URL=http://localhost:9988 tilt up`
// Analytics requests will show up in the http-echo-server window.

type analyticsOpter struct{}

var _ tiltanalytics.AnalyticsOpter = analyticsOpter{}

func (ao analyticsOpter) SetOpt(opt analytics.Opt) error {
	return analytics.SetOpt(opt)
}

func initAnalytics(rootCmd *cobra.Command) error {
	var analyticsCmd *cobra.Command
	var err error

	options := []analytics.Option{}
	// enabled: true because TiltAnalytics wraps the RemoteAnalytics and has its own guards for whether analytics
	//   is enabled. When TiltAnalytics decides to pass a call through to RemoteAnalytics, it should always work.
	options = append(options, analytics.WithGlobalTags(globalTags()), analytics.WithEnabled(true))
	analyticsURL := os.Getenv(analyticsURLEnvVar)
	if analyticsURL != "" {
		options = append(options, analytics.WithReportURL(analyticsURL))
	}
	backingAnalytics, analyticsCmd, err := analytics.Init(tiltAppName, options...)
	if err != nil {
		return err
	}

	rootCmd.AddCommand(analyticsCmd)

	if isAnalyticsDisabledFromEnv() {
		analyticsOpt = analytics.OptOut
	} else {
		analyticsOpt, err = analytics.OptStatus()
		if err != nil {
			return err
		}
	}

	analyticsService = tiltanalytics.NewTiltAnalytics(analyticsOpt, analyticsOpter{}, backingAnalytics)

	return nil
}

func globalTags() map[string]string {
	ret := map[string]string{
		"version": provideTiltInfo().AnalyticsVersion(),
		"os":      runtime.GOOS,
	}

	// store a hash of the git remote to help us guess how many users are running it on the same repository
	origin := normalizeGitRemote(gitOrigin("."))
	if origin != "" {
		h := md5.Sum([]byte(origin))
		ret["git.origin"] = base64.StdEncoding.EncodeToString(h[:])
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

func isAnalyticsDisabledFromEnv() bool {
	return os.Getenv(disableAnalyticsEnvVar) != ""
}

func provideAnalytics() (*tiltanalytics.TiltAnalytics, error) {
	if analyticsService == nil {
		return nil, fmt.Errorf("internal error: no available analytics service")
	}
	return analyticsService, nil
}
