package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/windmilleng/tilt/internal/build"
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
	browserMode string
	traceTags   string
}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <name> [<name2>] [<name3>] [...]",
		Short: "stand up one or more manifests",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.Flags().BoolVar(&c.watch, "watch", true, "If true, services will be automatically rebuilt and redeployed when files change. Otherwise, each service will be started once.")
	cmd.Flags().StringVar(&c.browserMode, "browser", "", "deprecated. TODO(nick): remove this flag")
	cmd.Flags().StringVar(&updateModeFlag, "update-mode", string(engine.UpdateModeAuto),
		fmt.Sprintf("Control the strategy Tilt uses for updating instances. Possible values: %v", engine.AllUpdateModes))
	cmd.Flags().StringVar(&c.traceTags, "traceTags", "", "tags to add to spans for easy querying, of the form: key1=val1,key2=val2")
	cmd.Flags().StringVar(&build.ImageTagPrefix, "image-tag-prefix", build.ImageTagPrefix,
		"For integration tests. Customize the image tag prefix so tests can write to a public registry")
	err := cmd.Flags().MarkHidden("image-tag-prefix")
	if err != nil {
		panic(err)
	}
	err = cmd.Flags().MarkHidden("browser")
	if err != nil {
		panic(err)
	}

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

	logOutput(fmt.Sprintf("Starting Tilt (%s)…\n", buildStamp()))

	if trace {
		traceID, err := tracer.TraceID(ctx)
		if err != nil {
			return err
		}
		logger.Get(ctx).Infof("TraceID: %s", traceID)
	}

	tf, err := tiltfile.Load(ctx, tiltfile.FileName)
	if err != nil {
		return err
	}

	manifests, err := tf.GetManifestConfigs(args...)
	if err != nil {
		return err
	}

	upper, err := wireUpper(ctx)
	if err != nil {
		return err
	}

	// Run the HUD in the background
	go func() {
		err := upper.RunHud(ctx)
		if err != nil {
			//TODO(matt) this might not be the best thing to do with an error - seems easy to miss
			logger.Get(ctx).Infof("error in hud: %v", err)
		}
	}()

	err = upper.CreateManifests(ctx, manifests, c.watch)
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
