package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/tracer"

	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/pkg/logger"
)

var debug bool
var verbose bool
var trace bool
var traceType string

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

	addCommand(rootCmd, &upCmd{})
	addCommand(rootCmd, &dockerCmd{})
	addCommand(rootCmd, &doctorCmd{})
	addCommand(rootCmd, newDownCmd())
	addCommand(rootCmd, &versionCmd{})
	addCommand(rootCmd, &dockerPruneCmd{})
	addCommand(rootCmd, newArgsCmd())

	rootCmd.AddCommand(analytics.NewCommand())
	rootCmd.AddCommand(newKubectlCmd())
	rootCmd.AddCommand(newDumpCmd(rootCmd))
	rootCmd.AddCommand(newTriggerCmd())
	rootCmd.AddCommand(newAlphaCmd())

	if len(os.Args) > 2 && os.Args[1] == "kubectl" {
		// Hack in global flags from kubectl
		flush := preKubectlCmdInit()
		defer flush()
	} else {
		globalFlags := rootCmd.PersistentFlags()
		globalFlags.BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
		globalFlags.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
		globalFlags.BoolVar(&trace, "trace", false, "Enable tracing")
		globalFlags.StringVar(&traceType, "traceBackend", "windmill", "Which tracing backend to use. Valid values are: 'windmill', 'lightstep', 'jaeger'")
		globalFlags.IntVar(&klogLevel, "klog", 0, "Enable Kubernetes API logging. Uses klog v-levels (0-4 are debug logs, 5-9 are tracing logs)")
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type tiltCmd interface {
	register() *cobra.Command
	run(ctx context.Context, args []string) error
}

func preCommand(ctx context.Context) (context.Context, func() error) {
	cleanup := func() error { return nil }
	l := logger.NewLogger(logLevel(verbose, debug), os.Stdout)
	ctx = logger.WithLogger(ctx, l)

	a, err := newAnalytics(l)
	if err != nil {
		l.Errorf("Fatal error initializing analytics: %v", err)
		os.Exit(1)
	}

	ctx = tiltanalytics.WithAnalytics(ctx, a)

	initKlog(l.Writer(logger.InfoLvl))

	if trace {
		backend, err := tracer.StringToTracerBackend(traceType)
		if err != nil {
			l.Warnf("invalid tracer backend: %v", err)
		}
		cleanup, err = tracer.Init(ctx, backend)
		if err != nil {
			l.Warnf("unable to initialize tracer: %s", err)
		}
	}

	// SIGNAL TRAPPING
	ctx, cancel := context.WithCancel(ctx)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs

		cancel()

		// If we get another SIGINT/SIGTERM, OR it takes too long for tilt to
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
		ctx, cleanup := preCommand(context.Background())

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
