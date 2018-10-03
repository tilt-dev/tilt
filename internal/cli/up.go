package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/windmilleng/tilt/internal/logger"

	"github.com/fatih/color"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/internal/tracer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var updateModeFlag string = string(engine.UpdateModeAuto)

type upCmd struct {
	watch       bool
	browserMode engine.BrowserMode
	traceTags   string
}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <name>",
		Short: "stand up a manifest",
		Args:  cobra.ExactArgs(1),
	}

	cmd.Flags().BoolVar(&c.watch, "watch", false, "any started manifests will be automatically rebuilt and redeployed when files in their repos change")
	cmd.Flags().Var(&c.browserMode, "browser", "open a browser when the manifest first starts")
	cmd.Flags().StringVar(&updateModeFlag, "update-mode", string(engine.UpdateModeAuto),
		fmt.Sprintf("Control the strategy Tilt uses for updating instances. Possible values: %v", engine.AllUpdateModes))
	cmd.Flags().StringVar(&c.traceTags, "traceTags", "", "tags to add to spans for easy querying, of the form: key1=val1,key2=val2")

	return cmd
}

func (c *upCmd) run(ctx context.Context, args []string) error {
	analyticsService.Incr("cmd.up", map[string]string{
		"watch": fmt.Sprintf("%v", c.watch),
		"mode":  string(updateModeFlag),
	})
	defer analyticsService.Flush(time.Second)

	span, ctx := opentracing.StartSpanFromContext(ctx, "Up")
	defer span.Finish()

	tags := tracer.TagStrToMap(c.traceTags)

	for k, v := range tags {
		span.SetTag(k, v)
	}

	logOutput(fmt.Sprintf("Starting Tilt (built %s)…\n", buildDateStamp()))

	if trace {
		traceID, err := tracer.TraceID(ctx)
		if err != nil {
			return err
		}
		logger.Get(ctx).Infof("TraceID: %s", traceID)
	}

	tf, err := tiltfile.Load(tiltfile.FileName, os.Stdout)
	if err != nil {
		return err
	}

	manifestName := args[0]
	manifests, err := tf.GetManifestConfigs(manifestName)
	if err != nil {
		return err
	}

	manifestCreator, err := wireManifestCreator(ctx, c.browserMode)
	if err != nil {
		return err
	}

	err = manifestCreator.CreateManifests(ctx, manifests, c.watch)
	s, ok := status.FromError(err)
	if ok && s.Code() == codes.Unknown {
		return errors.New(s.Message())
	} else if err != nil {
		if err == context.Canceled {
			// Expected case, no need to be loud about it, just exit
			return nil
		}
		return err
	}

	return nil
}

func logOutput(s string) {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	log.Printf(color.GreenString(s))
}

func provideUpdateModeFlag() engine.UpdateModeFlag {
	return engine.UpdateModeFlag(updateModeFlag)
}
