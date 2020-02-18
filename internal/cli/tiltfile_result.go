package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

const TiltfileErrExitCode = 5

type tiltfileResultDeps struct {
	tfl tiltfile.TiltfileLoader
}

func newTiltfileResultDeps(tfl tiltfile.TiltfileLoader) tiltfileResultDeps {
	return tiltfileResultDeps{
		tfl: tfl,
	}
}

func newTiltfileResultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tiltfile-result",
		Short: "exec the tiltfile and print results as json",

		Run: tiltfileResultPrintJSON,
	}

	cmd.Flags().StringVar(&fileName, "file", tiltfile.FileName, "Path to Tiltfile")

	return cmd
}

func tiltfileResultPrintJSON(cmd *cobra.Command, args []string) {
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx := logger.WithLogger(context.Background(), l)
	a, err := newAnalytics(l)
	if err != nil {
		failWithUnexpectedError(errors.Wrap(err, "Fatal error initializing analytics: %v"))
	}

	ctx = tiltanalytics.WithAnalytics(ctx, a)

	deps, err := wireTiltfileResult(ctx, a)
	if err != nil {
		failWithUnexpectedError(errors.Wrap(err, "wiring dependencies"))
	}

	tlr := deps.tfl.Load(ctx, fileName, model.NewUserConfigState(args))
	if tlr.Error != nil {
		// Some errors won't JSONify properly--instead of messing with that, print the error
		// and indicate what's going on via status code
		j, err := json.Marshal(struct{ Error string }{Error: tlr.Error.Error()})
		if err != nil {
			failWithUnexpectedError(errors.Wrap(err, "marshaling tlr.Error JSON"))
		}

		err = dumpJSON(bytes.NewReader(j))
		if err != nil {
			failWithUnexpectedError(errors.Wrap(err, "dump tlr.Error JSON"))
		}

		cmdFailWithCode(errors.Wrap(tlr.Error, "Tiltfile evaluated with error"), TiltfileErrExitCode)
	}

	j, err := json.Marshal(tlr)
	if err != nil {
		failWithUnexpectedError(errors.Wrap(err, "marshaling JSON"))
	}

	err = dumpJSON(bytes.NewReader(j))
	if err != nil {
		failWithUnexpectedError(errors.Wrap(err, "dump TiltfileLoadResult"))
	}
}

func failWithUnexpectedError(err error) {
	cmdFailWithCode(errors.Wrap(err, "unexpected error evaluating Tiltfile"), 1)
}
