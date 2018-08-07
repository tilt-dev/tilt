package cli

import (
	"fmt"

	"github.com/spf13/cobra"
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
	fmt.Printf("You ran 'up', go you!")
	return nil
}
