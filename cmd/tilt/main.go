package main

import (
	"github.com/windmilleng/tilt/internal/cli"
	_ "github.com/windmilleng/tilt/internal/tracer"
)

func main() {
	cli.Execute()
}
