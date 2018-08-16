package tiltd_server

import (
	"context"
	"os"
	"os/exec"

	"github.com/windmilleng/tilt/internal/tiltd"
)

type Daemon struct {
}

var _ tiltd.TiltD = &Daemon{}

func NewDaemon() (*Daemon, error) {
	return &Daemon{}, nil
}

func RunDaemon(ctx context.Context) (*os.Process, error) {
	cmd := exec.CommandContext(ctx, os.Args[0], "daemon")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return cmd.Process, nil
}
