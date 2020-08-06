package cli

import (
	"context"
	"log"
	"time"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/hud/server"

	"github.com/tilt-dev/tilt/internal/analytics"
)

type logsCmd struct {
	// TODO(maia): port
}

func (c *logsCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "logs [resource1, resource2...]",
		DisableFlagsInUseLine: true,
		Hidden:                true, // show when out of alpha
		Short:                 "stuff",
		Long: `
stuff and things
`,
	}
	// TODO(maia):
	//   - can pass port
	//   - pass resource names
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

	return server.StreamLogs(ctx, args, logDeps.url, logDeps.printer)
}
