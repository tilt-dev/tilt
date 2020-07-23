package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
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
	a := analytics.Get(ctx)
	machineID, _ := a.GlobalTag("machine")

	cmdVerifyInstallTags := engineanalytics.CmdTags(map[string]string{
		"machine": machineID,
	})
	a.Incr("cmd.verifyInstall", cmdVerifyInstallTags.AsMap())

	defer a.Flush(time.Second)

	return nil
}
