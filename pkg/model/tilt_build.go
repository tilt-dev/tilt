package model

import (
	"fmt"
)

// Information on a build of the Tilt binary
type TiltBuild struct {
	// Version w/o leading "v"
	Version string
	Date    string
	Dev     bool
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
	return WebVersion(fmt.Sprintf("v%s", b.Version))
}
