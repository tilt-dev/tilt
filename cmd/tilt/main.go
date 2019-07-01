package main

import (
	_ "expvar"

	"github.com/windmilleng/tilt/internal/cli"
	"github.com/windmilleng/tilt/internal/model"
)

// Magic variables set by goreleaser
var version string
var commit string
var date string

func main() {
	cli.SetTiltInfo(model.TiltBuild{
		Version: version,
		Date:    date,
	})
	cli.Execute()
}
