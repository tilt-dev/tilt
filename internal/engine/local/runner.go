package local

import (
	"context"
	"io"
	"time"

	"github.com/windmilleng/tilt/pkg/model"
)

// ServeSpec describes what Runner should be running
type ServeSpec struct {
	ManifestName model.ManifestName
	ServeCmd     model.Cmd
	TriggerTime  time.Time // TriggerTime is how Runner knows to restart; if it's newer than the TriggerTime of the currently running command, then Runner should restart it
}

type Runner interface {
	SetServeSpecs(ctx context.Context, specs []ServeSpec)
}

type Status int

const (
	Unknown Status = iota
	Running
	Done
	Error
)

type RunReporter interface {
	Writer(name model.ManifestName, sequenceNum int) io.Writer
	SetStatus(name model.ManifestName, status Status)
}

type FakeRunner struct {
}

func NewFakeRunner() *FakeRunner {
	return &FakeRunner{}
}
