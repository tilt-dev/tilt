package cli

import (
	"fmt"
	"os"
	"strings"
)

const devVersion = "0.7.1"

type BuildInfo struct {
	Version string
	Date    string
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
	return fmt.Sprintf("v%s, built %s", version, date)
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
		Date:    defaultBuildDate(),
		Version: fmt.Sprintf("%s-dev", devVersion),
	}
}
