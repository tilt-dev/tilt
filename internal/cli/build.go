package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/windmilleng/tilt/internal/model"
)

// Version for Go-compiled builds that didn't go through goreleaser.
//
// This is mostly here to make sure that people that use `go get` don't
// have a completely broken experience. It controls:
//
// 1) The version data you see when you run `tilt version`
// 2) The web JS you download when you're in web-mode prod.
//
// For distributed binaries, version is automatically baked
// into the binary with goreleaser. If this doesn't get updated
// on every release, it's often not that big a deal.
const devVersion = "0.7.7"

type BuildInfo struct {
	Version   string
	Date      string
	DevSuffix string
}

func (e BuildInfo) empty() bool {
	return e == BuildInfo{}
}

var globalBuildInfo BuildInfo

func SetBuildInfo(info BuildInfo) {
	globalBuildInfo = info
}

func buildInfo() BuildInfo {
	info := globalBuildInfo
	if info.empty() {
		return defaultBuildInfo()
	}
	return info
}

func buildStamp() string {
	info := buildInfo()
	version := info.Version
	date := info.Date
	timeIndex := strings.Index(date, "T")
	if timeIndex != -1 {
		date = date[0:timeIndex]
	}
	return fmt.Sprintf("v%s%s, built %s", version, info.DevSuffix, date)
}

// Returns a build datestamp in the format 2018-08-30
func defaultBuildDate() string {
	// TODO(nick): Add a mechanism to encode the datestamp in the binary with
	// ldflags. This currently only works if you are building your own
	// binaries. It won't work once we're distributing pre-built binaries.
	path, err := os.Executable()
	if err != nil {
		return "[unknown]"
	}

	info, err := os.Stat(path)
	if err != nil {
		return "[unknown]"
	}

	modTime := info.ModTime()
	return modTime.Format("2006-01-02")
}

// Returns a build datestamp in the format 2018-08-30
func defaultBuildInfo() BuildInfo {
	return BuildInfo{
		Date:      defaultBuildDate(),
		Version:   devVersion,
		DevSuffix: "-dev",
	}
}

func provideWebVersion() model.WebVersion {
	return model.WebVersion(fmt.Sprintf("v%s", buildInfo().Version))
}
