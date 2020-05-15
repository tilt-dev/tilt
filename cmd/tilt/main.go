package main

import (
	_ "expvar"

	"github.com/tilt-dev/tilt/internal/cli"
	"github.com/tilt-dev/tilt/pkg/model"
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
