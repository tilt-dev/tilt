package cli

import (
	"fmt"

	"context"

	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/tiltd"
)

type upCmd struct{}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "stand up a service",
	}

	return cmd
}

func (c *upCmd) run(args []string) error {
	fmt.Println("You ran 'up', go you!")
	proc, err := tiltd.RunDaemon(context.Background())
	if err != nil {
		return err
	}
	defer proc.Kill()

	dCli, err := tiltd.NewDaemonClient()
	if err != nil {
		return err
	}
	err = dCli.CreateService(context.Background(), "blahblahblah")
	if err != nil {
		fmt.Println("error calling the method")
		return err
	}

	return nil
}
