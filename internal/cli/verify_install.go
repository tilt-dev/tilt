package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/pkg/model"
)

type verifyInstallCmd struct {
}

func (c *verifyInstallCmd) name() model.TiltSubcommand { return "verify-install" }

func (c *verifyInstallCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify-install",
		Short: "Verifies Tilt Installation",
	}
	return cmd
}

// send info including "machine" tag to the wmstats db
func (c *verifyInstallCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.verifyInstall", nil)
	defer a.Flush(time.Second)

	return nil
}
