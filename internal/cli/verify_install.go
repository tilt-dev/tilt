package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type verifyInstallCmd struct {
}

func (c *verifyInstallCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify-install",
		Short: "Verifies Tilt Installation",
	}
	return cmd
}

func (c *verifyInstallCmd) run(ctx context.Context, args []string) error {
	fmt.Println("wohooo this cmd works>")
	return nil
}
