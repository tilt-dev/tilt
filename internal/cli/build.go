package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/windmilleng/tilt/pkg/model"
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
const devVersion = "0.12.8"

var commitSHA string
var globalTiltInfo model.TiltBuild

func SetTiltInfo(info model.TiltBuild) {
	globalTiltInfo = info
}

func tiltInfo() model.TiltBuild {
	info := globalTiltInfo
	if info.Empty() {
		return defaultTiltInfo()
	}
	return info
}

func buildStamp() string {
	info := tiltInfo()
	version := info.Version
	date := info.Date
	timeIndex := strings.Index(date, "T")
	if timeIndex != -1 {
		date = date[0:timeIndex]
	}
	devSuffix := ""
	if info.Dev {
		devSuffix = "-dev"
	}
	return fmt.Sprintf("v%s%s, built %s", version, devSuffix, date)
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
func defaultTiltInfo() model.TiltBuild {
	return model.TiltBuild{
		Date:      defaultBuildDate(),
		Version:   devVersion,
		CommitSHA: commitSHA,
		Dev:       true,
	}
}

func provideTiltInfo() model.TiltBuild {
	return tiltInfo()
}

func provideWebVersion(b model.TiltBuild) model.WebVersion {
	return b.WebVersion()
}
