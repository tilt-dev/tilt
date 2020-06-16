package model

import (
	"fmt"
	"strings"
)

// Information on a build of the Tilt binary
type TiltBuild struct {
	// Version w/o leading "v"
	Version   string
	CommitSHA string
	Date      string
	Dev       bool
}

func (b TiltBuild) Empty() bool {
	return b == TiltBuild{}
}

func (b TiltBuild) AnalyticsVersion() string {
	if b.Dev {
		return b.Version + "-dev"
	}

	return b.Version
}

func (b TiltBuild) WebVersion() WebVersion {
	v := fmt.Sprintf("v%s", b.Version)
	return WebVersion(v)
}

func (b TiltBuild) HumanBuildStamp() string {
	version := b.Version
	date := b.Date
	timeIndex := strings.Index(date, "T")
	if timeIndex != -1 {
		date = date[0:timeIndex]
	}
	devSuffix := ""
	if b.Dev {
		devSuffix = "-dev"
	}
	return fmt.Sprintf("v%s%s, built %s", version, devSuffix, date)
}
