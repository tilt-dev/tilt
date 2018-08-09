package cli

import (
		"context"

	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_server"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_client"
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
	serviceYaml, err := tf.GetServiceConfig(serviceName)
	if err != nil {
		return err
	}

	err = dCli.CreateService(ctx, *serviceYaml)
	if err != nil {
		return err
	}

	return nil
}
