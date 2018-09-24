package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/windmilleng/tilt/internal/output"

	"github.com/windmilleng/tilt/internal/tracer"

	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/logger"
)

var debug bool
var verbose bool
var trace bool

func logLevel() logger.Level {
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

	addCommand(rootCmd, &upCmd{browserMode: engine.BrowserAuto})
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	rootCmd.PersistentFlags().BoolVar(&trace, "trace", false, "Enable tracing")

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
	l := logger.NewLogger(logLevel(), os.Stdout)
	ctx = context.Background()
	ctx = output.WithOutputter(
		logger.WithLogger(ctx, l),
		output.NewOutputter(l))

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
		_ = <-sigs

		// We rely on context cancellation being handled elsewhere --
		// otherwise there's no way to SIGINT/SIGTERM this app o_0
		cancel()
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
