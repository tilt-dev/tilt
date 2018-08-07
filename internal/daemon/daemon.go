package daemon

import (
	"context"
	"fmt"
)

const Port = 10000

type Daemon struct{}

func NewDaemon() (*Daemon, error) {
	return &Daemon{}, nil
}

func (d *Daemon) CreateService(ctx context.Context, k8sYaml string) error {
	fmt.Println("I made you a service, hope you like it!")
	return nil
}
