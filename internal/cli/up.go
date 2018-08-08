package cli

import (
	"fmt"

	"context"

	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_server"
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
	ctx := context.Background()
	proc, err := tiltd_server.RunDaemon(ctx)
	if err != nil {
		return err
	}
	defer proc.Kill()

	return nil
}
