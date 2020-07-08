package cli

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/tilt-dev/tilt/internal/hud/server"

	"github.com/tilt-dev/tilt/internal/analytics"
)

type logsCmd struct {
	// port?
}

func (c *logsCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "logs [resource1, resource2...]",
		DisableFlagsInUseLine: true,
		Short:                 "stuff",
		Long: `
stuff and things
`,
	}

	return cmd
}

func (c *logsCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)

	a.Incr("cmd.logs", nil)
	defer a.Flush(time.Second)

	span, ctx := opentracing.StartSpanFromContext(ctx, "Up")
	defer span.Finish()

	if ok, reason := analytics.IsAnalyticsDisabledFromEnv(); ok {
		log.Printf("Tilt analytics disabled: %s", reason)
	}

	fmt.Println("you ran a command, go you!")

	reader := server.ProvideWebsockerReader()
	reader.Listen()

	return nil
}
