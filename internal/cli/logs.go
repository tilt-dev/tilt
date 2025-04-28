package cli

import (
	"context"
	"log"
	"time"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/analytics"
)

type logsCmd struct {
	follow bool // if true, follow logs (otherwise print current logs and exit)
}

func (c *logsCmd) name() model.TiltSubcommand { return "logs" }

func (c *logsCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "logs [resource1, resource2...]",
		DisableFlagsInUseLine: true,
		Short:                 "Get logs from a running Tilt instance (optionally filtered for the specified resources)",
		Long: `Get logs from a running Tilt instance (optionally filtered for the specified resources).

By default, looks for a running Tilt instance on localhost:10350
(this is configurable with the --port and --host flags).
`,
	}

	cmd.Flags().BoolVarP(&c.follow, "follow", "f", false, "If true, stream the requested logs; otherwise, print the requested logs at the current moment in time, then exit.")

	addConnectServerFlags(cmd)
	addLogFilterFlags(cmd, "")
	return cmd
}

func (c *logsCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)

	a.Incr("cmd.logs", nil)
	defer a.Flush(time.Second)

	if ok, reason := analytics.IsAnalyticsDisabledFromEnv(); ok {
		log.Printf("Tilt analytics disabled: %s", reason)
	}

	// For `tilt logs`, the resources are passed as extra args.
	logResourcesFlag = args

	logDeps, err := wireLogsDeps(ctx, a, "logs")
	if err != nil {
		return err
	}

	return server.StreamLogs(ctx, c.follow, logDeps.url, logDeps.filter, logDeps.printer)
}
