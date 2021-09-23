package cli

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/tilt/pkg/logger"

	"github.com/tilt-dev/tilt/internal/analytics"
	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/apis/tiltfile"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/model"
)

var tupleRE = regexp.MustCompile(`,\)$`)

// arbitrary non-1 value chosen to allow callers to distinguish between
// Tilt errors and Tiltfile errors
const TiltfileErrExitCode = 5

type tiltfileResultCmd struct {
	streams genericclioptions.IOStreams
	exit    func(code int)

	fileName string

	// for Builtin Timings mode
	builtinTimings bool
	durThreshold   time.Duration
}

var _ tiltCmd = &tiltfileResultCmd{}

type cmdTiltfileResultDeps struct {
	tfl tiltfile.TiltfileLoader
}

func newTiltfileResultDeps(tfl tiltfile.TiltfileLoader) cmdTiltfileResultDeps {
	return cmdTiltfileResultDeps{
		tfl: tfl,
	}
}

func newTiltfileResultCmd() *tiltfileResultCmd {
	return &tiltfileResultCmd{
		streams: genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin},
		exit:    os.Exit,
	}
}

func (c *tiltfileResultCmd) name() model.TiltSubcommand { return "tiltfile-result" }

func (c *tiltfileResultCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tiltfile-result",
		Short: "Exec the Tiltfile and print data about execution",
		Long: `Exec the Tiltfile and print data about execution.

By default, prints Tiltfile execution results as JSON (note: the API is unstable and may change); can also print timings of Tiltfile Builtin calls.

Exit code 0: successful Tiltfile evaluation (data printed to stdout)
Exit code 1: some failure in setup, printing results, etc. (any logs printed to stderr)
Exit code 5: error when evaluating the Tiltfile, such as syntax error, illegal Tiltfile operation, etc. (any logs printed to stderr)

Run with -v | --verbose to print Tiltfile execution logs on stderr, regardless of whether there was an error.`,
	}

	addTiltfileFlag(cmd, &c.fileName)
	addKubeContextFlag(cmd)
	cmd.Flags().BoolVarP(&c.builtinTimings, "builtin-timings", "b", false, "If true, print timing data for Tiltfile builtin calls instead of Tiltfile result JSON")
	cmd.Flags().DurationVar(&c.durThreshold, "dur-threshold", 0, "Only compatible with Builtin Timings mode. Should be a Go duration string. If passed, only print information about builtin calls lasting this duration and longer.")

	return cmd
}

func (c *tiltfileResultCmd) run(ctx context.Context, args []string) error {
	// HACK(maia): we're overloading the -v|--verbose flags here, which isn't ideal,
	// but eh, it's fast. Might be cleaner to do --logs=true or something.
	logLvl := logger.Get(ctx).Level()
	showTiltfileLogs := logLvl.ShouldDisplay(logger.VerboseLvl)

	if !showTiltfileLogs {
		// defer Tiltfile output -- only print on error
		l := logger.NewDeferredLogger(ctx)
		ctx = logger.WithLogger(ctx, l)
	} else {
		// send all logs to stderr so stdout has only structured output
		ctx = logger.WithLogger(ctx, logger.NewLogger(logLvl, c.streams.ErrOut))
	}

	deps, err := wireTiltfileResult(ctx, analytics.Get(ctx), "alpha tiltfile-result")
	if err != nil {
		c.maybePrintDeferredLogsToStderr(ctx, showTiltfileLogs)
		return errors.Wrap(err, "wiring dependencies")
	}

	start := time.Now()
	tlr := deps.tfl.Load(ctx, ctrltiltfile.MainTiltfile(c.fileName, args))
	tflDur := time.Since(start)
	if tlr.Error != nil {
		c.maybePrintDeferredLogsToStderr(ctx, showTiltfileLogs)

		// Some errors won't JSONify properly by default, so just print it
		// to STDERR and use the exit code to indicate that it's an error
		// from Tiltfile parsing.
		fmt.Fprintln(c.streams.ErrOut, tlr.Error)
		c.exit(TiltfileErrExitCode)
		return nil
	}

	// Instead of printing result JSON, print Builtin Timings instead
	if c.builtinTimings {
		if len(tlr.BuiltinCalls) == 0 {
			return fmt.Errorf("executed Tiltfile, but recorded no Builtin calls")
		}
		for _, call := range tlr.BuiltinCalls {
			if call.Dur < c.durThreshold {
				continue
			}
			argsStr := tupleRE.ReplaceAllString(fmt.Sprintf("%v", call.Args), ")") // clean up tuple stringification
			fmt.Fprintf(c.streams.Out, "- %s%s took %s\n", call.Name, argsStr, call.Dur)
		}
		fmt.Fprintf(c.streams.Out, "Tiltfile execution took %s\n", tflDur.String())
		return nil
	}

	err = encodeJSON(c.streams.Out, tlr)
	if err != nil {
		c.maybePrintDeferredLogsToStderr(ctx, showTiltfileLogs)
		return errors.Wrap(err, "encoding JSON")
	}
	return nil
}

func (c *tiltfileResultCmd) maybePrintDeferredLogsToStderr(ctx context.Context, showTiltfileLogs bool) {
	if showTiltfileLogs {
		// We've already printed the logs elsewhere, do nothing
		return
	}
	l, ok := logger.Get(ctx).(*logger.DeferredLogger)
	if !ok {
		panic(fmt.Sprintf("expected logger of type DeferredLogger, got: %T", logger.Get(ctx)))
	}
	stderrLogger := logger.NewLogger(l.Level(), c.streams.ErrOut)
	l.SetOutput(stderrLogger)
}
