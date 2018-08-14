package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_client"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_server"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

type upCmd struct{}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <servicename>",
		Short: "stand up a service",
		Args:  cobra.ExactArgs(1),
	}

	return cmd
}

func (c *upCmd) run(args []string) error {
	ctx := context.Background()
	proc, err := tiltd_server.RunDaemon(ctx)
	if err != nil {
		return err
	}
	defer proc.Kill()

	dCli, err := tiltd_client.NewDaemonClient()
	if err != nil {
		return err
	}

	tf, err := tiltfile.Load("Tiltfile")
	if err != nil {
		return err
	}

	serviceName := args[0]
	service, err := tf.GetServiceConfig(serviceName)
	if err != nil {
		return err
	}

	err = dCli.CreateService(ctx, *service)
	if err != nil {
		return err
	}

	if debug {
		fmt.Println("HI PAT AND HAN!!!!!!")
	}
	return nil
}
