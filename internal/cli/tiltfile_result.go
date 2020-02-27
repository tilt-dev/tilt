package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/pkg/logger"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/pkg/model"
)

// arbitrary non-1 value chosen to allow callers to distinguish between
// Tilt errors and Tiltfile errors
const TiltfileErrExitCode = 5

type tiltfileResultCmd struct {
	fileName string
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
	return &tiltfileResultCmd{}
}

func (c *tiltfileResultCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tiltfile-result",
		Short: "Exec the Tiltfile and print results as JSON (note: the API is unstable and may change)",
		Long: `Exec the Tiltfile and print results as JSON (note: the API is unstable and may change).

Exit code 0: successful Tiltfile evaluation (JSON printed to stdout)
Exit code 1: some failure in setup, printing results, etc. (any logs printed to stderr)
Exit code 5: error when evaluating the Tiltfile, such as syntax error, illegal Tiltfile operation, etc. (any logs printed to stderr)

Run with -v | --verbose to print Tiltfile execution logs on stderr, regardless of whether there was an error.`,
	}

	cmd.Flags().StringVar(&c.fileName, "file", tiltfile.FileName, "Path to Tiltfile")

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
		ctx = logger.WithLogger(ctx, logger.NewLogger(logLvl, os.Stderr))
	}

	deps, err := wireTiltfileResult(ctx, analytics.Get(ctx))
	if err != nil {
		maybePrintDeferredLogsToStderr(ctx, showTiltfileLogs)
		return errors.Wrap(err, "wiring dependencies")
	}

	tlr := deps.tfl.Load(ctx, c.fileName, model.NewUserConfigState(args))
	if tlr.Error != nil {
		maybePrintDeferredLogsToStderr(ctx, showTiltfileLogs)

		// Some errors won't JSONify properly by default, so just print it
		// to STDERR and use the exit code to indicate that it's an error
		// from Tiltfile parsing.
		fmt.Fprintln(os.Stderr, tlr.Error)
		os.Exit(TiltfileErrExitCode)
	}

	err = encodeJSON(tlr)
	if err != nil {
		maybePrintDeferredLogsToStderr(ctx, showTiltfileLogs)
		return errors.Wrap(err, "encoding JSON")
	}
	return nil
}

func maybePrintDeferredLogsToStderr(ctx context.Context, showTiltfileLogs bool) {
	if showTiltfileLogs {
		// We've already printed the logs elsewhere, do nothing
		return
	}
	l, ok := logger.Get(ctx).(*logger.DeferredLogger)
	if !ok {
		panic(fmt.Sprintf("expected logger of type DeferredLogger, got: %T", logger.Get(ctx)))
	}
	stderrLogger := logger.NewLogger(l.Level(), os.Stderr)
	l.SetOutput(stderrLogger)
}
