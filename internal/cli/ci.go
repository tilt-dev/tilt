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
	"github.com/tilt-dev/tilt/internal/cloud"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type ciCmd struct {
	fileName             string
	outputSnapshotOnExit string
}

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
`, DefaultWebHost, DefaultWebPort),
	}

	addStartServerFlags(cmd)
	addDevServerFlags(cmd)
	addTiltfileFlag(cmd, &c.fileName)

	cmd.Flags().BoolVar(&logActionsFlag, "logactions", false, "log all actions and state changes")
	cmd.Flags().Lookup("logactions").Hidden = true
	cmd.Flags().StringVar(&c.outputSnapshotOnExit, "output-snapshot-on-exit", "",
		"If specified, Tilt will dump a snapshot of its state to the specified path when it exits")

	return cmd
}

func (c *ciCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.ci", nil)
	defer a.Flush(time.Second)

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

	logOutput(fmt.Sprintf("Starting Tilt (%s)…", buildStamp()))

	if ok, reason := analytics.IsAnalyticsDisabledFromEnv(); ok {
		log.Printf("Tilt analytics disabled: %s", reason)
	}

	// TODO(nick): Make this better than a global variable.
	noBrowser = true

	cmdCIDeps, err := wireCmdCI(ctx, a)
	if err != nil {
		deferred.SetOutput(deferred.Original())
		return err
	}

	upper := cmdCIDeps.Upper

	l := store.NewLogActionLogger(ctx, upper.Dispatch)
	deferred.SetOutput(l)
	ctx = redirectLogs(ctx, l)
	if c.outputSnapshotOnExit != "" {
		defer cloud.WriteSnapshot(ctx, cmdCIDeps.Store, c.outputSnapshotOnExit)
	}

	engineMode := store.EngineModeCI

	err = upper.Start(ctx, args, cmdCIDeps.TiltBuild, engineMode,
		c.fileName, false, a.UserOpt(), cmdCIDeps.Token,
		string(cmdCIDeps.CloudAddress))
	if err == nil {
		_, _ = fmt.Fprintln(colorable.NewColorableStdout(),
			color.GreenString("SUCCESS. All workloads are healthy."))
	}
	return err
}
