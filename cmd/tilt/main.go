package main

import (
	"log"

	"github.com/windmilleng/tilt/internal/cli"
	"github.com/windmilleng/tilt/internal/tracer"
)

func main() {
	traceCloser, err := tracer.Init()
	if err != nil {
		log.Printf("Warning: unable to initialize tracer: %s", err)
	}
	cli.Execute(traceCloser.Close)
}
