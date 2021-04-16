package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tilt-dev/wmclient/pkg/analytics"
	"go.opencensus.io/stats"

	"github.com/tilt-dev/tilt/pkg/model"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/output"
	"github.com/tilt-dev/tilt/pkg/logger"
)

var debug bool
var verbose bool

func logLevel(verbose, debug bool) logger.Level {
	if debug {
		return logger.DebugLvl
	} else if verbose {
		return logger.VerboseLvl
	} else {
		return logger.InfoLvl
	}
}

func Execute() {
	err := readEnvDefaults()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rootCmd := &cobra.Command{
		Use:   "tilt",
		Short: "Multi-service development with no stress",
		Long: `
Tilt helps you develop your microservices locally.
Run 'tilt up' to start working on your services in a complete dev environment
configured for your team.

Tilt watches your files for edits, automatically builds your container images,
and applies any changes to bring your environment
up-to-date in real-time. Think 'docker build && kubectl apply' or 'docker-compose up'.
`,
	}

	addCommand(rootCmd, &ciCmd{})
	addCommand(rootCmd, &upCmd{})
	addCommand(rootCmd, &dockerCmd{})
	addCommand(rootCmd, &doctorCmd{})
	addCommand(rootCmd, newDownCmd())
	addCommand(rootCmd, &versionCmd{})
	addCommand(rootCmd, &verifyInstallCmd{})
	addCommand(rootCmd, &dockerPruneCmd{})
	addCommand(rootCmd, newArgsCmd())
	addCommand(rootCmd, &logsCmd{})
	addCommand(rootCmd, newGetCmd())
	addCommand(rootCmd, newEditCmd())
	addCommand(rootCmd, newApiresourcesCmd())
	addCommand(rootCmd, newDeleteCmd())
	addCommand(rootCmd, newApplyCmd())
	addCommand(rootCmd, newCreateCmd())

	rootCmd.AddCommand(analytics.NewCommand())
	rootCmd.AddCommand(newDumpCmd(rootCmd))
	rootCmd.AddCommand(newTriggerCmd())
	rootCmd.AddCommand(newAlphaCmd())

	globalFlags := rootCmd.PersistentFlags()
	globalFlags.BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	globalFlags.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	globalFlags.IntVar(&klogLevel, "klog", 0, "Enable Kubernetes API logging. Uses klog v-levels (0-4 are debug logs, 5-9 are tracing logs)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type tiltCmd interface {
	name() model.TiltSubcommand
	register() *cobra.Command
	run(ctx context.Context, args []string) error
}

func preCommand(ctx context.Context, cmdName model.TiltSubcommand) (context.Context, func() error) {
	l := logger.NewLogger(logLevel(verbose, debug), os.Stdout)
	ctx = logger.WithLogger(ctx, l)

	ctx, cleanup, err := initMetrics(ctx, cmdName)
	if err != nil {
		l.Errorf("Fatal error initializing metrics: %v", err)
		os.Exit(1)
	}

	stats.Record(ctx, CommandCountMeasure.M(1))

	a, err := wireAnalytics(l, cmdName)
	if err != nil {
		l.Errorf("Fatal error initializing analytics: %v", err)
		os.Exit(1)
	}

	ctx = tiltanalytics.WithAnalytics(ctx, a)

	initKlog(l.Writer(logger.InfoLvl))

	// SIGNAL TRAPPING
	ctx, cancel := context.WithCancel(ctx)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigs

		cancel()

		// If we get another signal, OR it takes too long for tilt to
		// exit after canceling context, just exit
		select {
		case <-sigs:
			l.Debugf("force quitting...")
			os.Exit(1)
		case <-time.After(2 * time.Second):
			l.Debugf("Context canceled but app still running; forcibly exiting.")
			os.Exit(1)
		}
	}()

	return ctx, cleanup
}

func addCommand(parent *cobra.Command, child tiltCmd) {
	cobraChild := child.register()
	cobraChild.Run = func(_ *cobra.Command, args []string) {
		ctx, cleanup := preCommand(context.Background(), child.name())

		err := child.run(ctx, args)

		err2 := cleanup()
		// ignore cleanup errors if we have a real error
		if err == nil {
			err = err2
		}

		if err != nil {
			// TODO(maia): this shouldn't print if we've already pretty-printed it
			_, printErr := fmt.Fprintf(output.OriginalStderr, "Error: %v\n", err)
			if printErr != nil {
				panic(printErr)
			}
			os.Exit(1)
		}
	}

	parent.AddCommand(cobraChild)
}
