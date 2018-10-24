package main

import (
	"github.com/windmilleng/tilt/internal/cli"
)

// Magic variables set by goreleaser
var version string
var commit string
var date string

func main() {
	cli.SetBuildInfo(cli.BuildInfo{
		Version: version,
		Date:    date,
	})
	cli.Execute()
}
