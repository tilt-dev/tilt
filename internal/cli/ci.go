package cli

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ciCmd struct {
	fileName             string
	outputSnapshotOnExit string
}

func (c *ciCmd) name() model.TiltSubcommand { return "ci" }

func (c *ciCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "ci [<tilt flags>] [-- <Tiltfile args>]",
		DisableFlagsInUseLine: true,
		Short:                 "Start Tilt in CI/batch mode with the given Tiltfile args",
		Long: fmt.Sprintf(`
Starts Tilt and runs resources defined in the Tiltfile.

Exits with failure if any build fails or any server crashes.

Exits with success if all tasks have completed successfully
and all servers are healthy.

While Tilt is running, you can view the UI at %s:%d
(configurable with --host and --port).

See blog post for additional information: https://blog.tilt.dev/2020/04/16/how-to-not-break-server-startup.html
`, defaultWebHost, defaultWebPort),
	}

	addStartServerFlags(cmd)
	addDevServerFlags(cmd)
	addTiltfileFlag(cmd, &c.fileName)
	addKubeContextFlag(cmd)
	addNamespaceFlag(cmd)
	addLogFilterFlags(cmd, "log-")
	addLogFilterResourcesFlag(cmd)

	cmd.Flags().BoolVar(&logActionsFlag, "logactions", false, "log all actions and state changes")
	cmd.Flags().Lookup("logactions").Hidden = true
	cmd.Flags().StringVar(&c.outputSnapshotOnExit, "output-snapshot-on-exit", "",
		"If specified, Tilt will dump a snapshot of its state to the specified path when it exits")
	cmd.Flags().DurationVar(&ciTimeout, "timeout", model.CITimeoutDefault,
		"Timeout to wait for CI to pass. Set to 0 for no timeout.")

	return cmd
}

func (c *ciCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.ci", nil)
	defer a.Flush(time.Second)

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	webHost := provideWebHost()
	webURL, _ := provideWebURL(webHost, provideWebPort())
	startLine := prompt.StartStatusLine(webURL, webHost)
	log.Print(startLine)
	log.Print(buildStamp())

	if ok, reason := analytics.IsAnalyticsDisabledFromEnv(); ok {
		log.Printf("Tilt analytics disabled: %s", reason)
	}

	cmdCIDeps, err := wireCmdCI(ctx, a, "ci")
	if err != nil {
		deferred.SetOutput(deferred.Original())
		return err
	}

	upper := cmdCIDeps.Upper

	l := store.NewLogActionLogger(ctx, upper.Dispatch)
	deferred.SetOutput(l)
	ctx = redirectLogs(ctx, l)
	if c.outputSnapshotOnExit != "" {
		defer cmdCIDeps.Snapshotter.WriteSnapshot(ctx, c.outputSnapshotOnExit)
	}

	err = upper.Start(ctx, args, cmdCIDeps.TiltBuild,
		c.fileName, store.TerminalModeStream, a.UserOpt(), cmdCIDeps.Token,
		string(cmdCIDeps.CloudAddress))
	if err == nil {
		_, _ = fmt.Fprintln(colorable.NewColorableStdout(),
			color.GreenString("SUCCESS. All workloads are healthy."))
	}
	return err
}

var ciTimeout time.Duration
