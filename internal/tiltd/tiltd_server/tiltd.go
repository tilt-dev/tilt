package tiltd_server

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/windmilleng/tilt/internal/tiltd"
)

type Daemon struct{}

var _ tiltd.TiltD = &Daemon{}

func NewDaemon() (*Daemon, error) {
	return &Daemon{}, nil
}

func (d *Daemon) CreateService(ctx context.Context, k8sYaml string) error {
	fmt.Println("I made you a service, hope you like it!")
	//return fmt.Errorf("this is an error from your daemon")
	return nil
}

func RunDaemon(ctx context.Context) (*os.Process, error) {
	// relies on latest tiltd installed
	cmd := exec.CommandContext(ctx, "tiltd")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("hi i'm running ur daemon")
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return cmd.Process, nil
}
