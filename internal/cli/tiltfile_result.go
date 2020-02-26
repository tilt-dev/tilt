package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

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
	}

	cmd.Flags().StringVar(&c.fileName, "file", tiltfile.FileName, "Path to Tiltfile")

	return cmd
}

func (c *tiltfileResultCmd) run(ctx context.Context, args []string) error {
	deps, err := wireTiltfileResult(ctx, analytics.Get(ctx))
	if err != nil {
		return errors.Wrap(err, "wiring dependencies")
	}

	tlr := deps.tfl.Load(ctx, c.fileName, model.NewUserConfigState(args))
	if tlr.Error != nil {
		// Some errors won't JSONify properly by default, so just print it
		// to STDERR and use the exit code to indicate that it's an error
		// from Tiltfile parsing.
		fmt.Fprintln(os.Stderr, tlr.Error)
		os.Exit(TiltfileErrExitCode)
	}

	err = encodeJSON(tlr, os.Stderr)
	if err != nil {
		return errors.Wrap(err, "encoding JSON")
	}
	return nil
}
