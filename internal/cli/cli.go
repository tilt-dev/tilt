package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/windmilleng/tilt/internal/tracer"

	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/internal/logger"
)

var debug bool
var verbose bool
var trace bool

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

	err := initAnalytics(rootCmd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	addCommand(rootCmd, &upCmd{})
	addCommand(rootCmd, &doctorCmd{})
	addCommand(rootCmd, &downCmd{})
	addCommand(rootCmd, &demoCmd{})
	addCommand(rootCmd, &versionCmd{})

	globalFlags := rootCmd.PersistentFlags()
	globalFlags.BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	globalFlags.BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	globalFlags.BoolVar(&trace, "trace", false, "Enable tracing")
	globalFlags.IntVar(&klogLevel, "klog", 0, "Enable Kubernetes API logging. Uses klog v-levels (0-4 are debug logs, 5-9 are tracing logs)")
	err = globalFlags.MarkHidden("klog")
	if err != nil {
		panic(err)
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
	initKlog()
	l := logger.NewLogger(logLevel(verbose, debug), os.Stdout)
	ctx = logger.WithLogger(ctx, l)

	if trace {
		var err error
		cleanup, err = tracer.Init(ctx)
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
			_, printErr := fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			if printErr != nil {
				panic(printErr)
			}
			os.Exit(1)
		}
	}

	parent.AddCommand(cobraChild)
}
