package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/pkg/model"
)

type versionCmd struct {
}

func (c *versionCmd) name() model.TiltSubcommand { return "version" }

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
