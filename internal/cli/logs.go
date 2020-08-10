package cli

import (
	"context"
	"log"
	"time"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/hud/server"

	"github.com/tilt-dev/tilt/internal/analytics"
)

type logsCmd struct{}

func (c *logsCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "logs [resource1, resource2...]",
		DisableFlagsInUseLine: true,
		Short:                 "stream logs from a running Tilt instance (optionally filtered for the specified resources)",
		Long: `Stream logs from a running Tilt instance (optionally filtered for the specified resources).

By default, looks for a running Tilt instance on localhost:10350
(this is configurable with the --port and --host flags).
`,
	}

	// TODO: log level flags
	addConnectServerFlags(cmd)
	return cmd
}

func (c *logsCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)

	a.Incr("cmd.logs", nil)
	defer a.Flush(time.Second)

	if ok, reason := analytics.IsAnalyticsDisabledFromEnv(); ok {
		log.Printf("Tilt analytics disabled: %s", reason)
	}

	logDeps, err := wireLogsDeps(ctx, a, "logs")
	if err != nil {
		return err
	}

	return server.StreamLogs(ctx, logDeps.url, args, logDeps.printer)
}
