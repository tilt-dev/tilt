package main

import (
	"log"

	"github.com/windmilleng/tilt/internal/cli"
	"github.com/windmilleng/tilt/internal/tracer"
)

func main() {
	err := tracer.Init()
	dummy := func() error { return nil }
	if err != nil {
		log.Printf("Warning: unable to initialize tracer: %s", err)
	}
	cli.Execute(dummy)
}
