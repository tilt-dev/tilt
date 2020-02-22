package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type versionCmd struct {
}

func (c *versionCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Current Tilt version",
	}
	return cmd
}

func (c *versionCmd) run(ctx context.Context, args []string) error {
	fmt.Println(buildStamp())
	return nil
}
