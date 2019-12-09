package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/windmilleng/tilt/internal/analytics"
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
		Short: "tilt creates Kubernetes Live Deploys that reflect changes seconds after they’re made",
	}

	a, err := initAnalytics(rootCmd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	addCommand(rootCmd, &upCmd{}, a)
	addCommand(rootCmd, &dockerCmd{}, a)
	addCommand(rootCmd, &doctorCmd{}, a)
	addCommand(rootCmd, &downCmd{}, a)
	addCommand(rootCmd, &versionCmd{}, a)
	addCommand(rootCmd, &dockerPruneCmd{}, a)

	rootCmd.AddCommand(newKubectlCmd())
	rootCmd.AddCommand(newDumpCmd())

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

func preCommand(ctx context.Context, a *analytics.TiltAnalytics) (context.Context, func() error) {
	cleanup := func() error { return nil }
	l := logger.NewLogger(logLevel(verbose, debug), os.Stdout)
	ctx = logger.WithLogger(ctx, l)
	ctx = analytics.WithAnalytics(ctx, a)

	initKlog(l.Writer(logger.InfoLvl))

	// if dir, err := dirs.UseWindmillDir(); err == nil {
	// 	// TODO(dbentley): we can't error here, which is frustrating
	// }

	// if err := tracer.InitOpenTelemetry(dir); err != nil {
	// 	log.Printf("Warning: error setting up open telemetry: %v", error)
	// }

	if trace {
		backend, err := tracer.StringToTracerBackend(traceType)
		if err != nil {
			log.Printf("Warning: invalid tracer backend: %v", err)
		}
		cleanup, err = tracer.Init(ctx, backend)
		if err != nil {
			log.Printf("Warning: unable to initialize tracer: %s", err)
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
		// exit after cancelling context, just exit
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

func addCommand(parent *cobra.Command, child tiltCmd, a *analytics.TiltAnalytics) {
	cobraChild := child.register()
	cobraChild.Run = func(_ *cobra.Command, args []string) {
		ctx, cleanup := preCommand(context.Background(), a)

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
