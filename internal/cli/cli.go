package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tilt-dev/tilt/internal/output"

	"github.com/tilt-dev/tilt/internal/tiltfile/config"

	"github.com/spf13/cobra"
	"github.com/tilt-dev/wmclient/pkg/analytics"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/tracer"
	"github.com/tilt-dev/tilt/pkg/logger"
)

var debug bool
var verbose bool
var trace bool
var traceType string
var subcommand string

func logLevel(verbose, debug bool) logger.Level {
	if debug {
		return logger.DebugLvl
	} else if verbose {
		return logger.VerboseLvl
	} else {
		return logger.InfoLvl
	}
}

func Cmd() *cobra.Command {
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

	return rootCmd
}

type ExitCodeError struct {
	ExitCode int
	Err      error
}

func (ece ExitCodeError) Error() string {
	return ece.Err.Error()
}

func Execute() {
	cmd := Cmd()

	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(output.OriginalStderr, err)

		exitCode := 1
		if ece, ok := err.(ExitCodeError); ok {
			exitCode = ece.ExitCode
		}

		os.Exit(exitCode)
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

// e.g., for 'tilt alpha tiltfile-result', return 'alpha tiltfile-result'
func fullSubcommandString(cmd *cobra.Command) string {
	cmdPieces := []string{cmd.Name()}
	for p := cmd.Parent(); p != nil; p = p.Parent() {
		cmdPieces = append([]string{p.Name()}, cmdPieces...)
	}

	// skip the first piece, i.e. "tilt"
	return strings.Join(cmdPieces[1:], " ")
}

func addCommand(parent *cobra.Command, child tiltCmd) {
	cobraChild := child.register()
	cobraChild.RunE = func(_ *cobra.Command, args []string) error {
		subcommand = fullSubcommandString(cobraChild)
		// by default, cobra prints usage on any kind of error
		// if we've made it this far, we're past arg-parsing, so an error is not likely to be
		// a usage error, so printing usage isn't appropriate
		// cobra doesn't support this well: https://github.com/spf13/cobra/issues/340#issuecomment-374617413
		cobraChild.SilenceUsage = true

		ctx, cleanup := preCommand(context.Background())

		err := child.run(ctx, args)

		err2 := cleanup()
		// ignore cleanup errors if we have a real error
		if err == nil {
			err = err2
		}

		return err
	}

	parent.AddCommand(cobraChild)
}

func provideTiltSubcommand() config.TiltSubcommand {
	return config.TiltSubcommand(subcommand)
}
