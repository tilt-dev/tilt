package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/tracer"
)

type hudCmd struct {
	traceTags string
}

func (c *hudCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hud",
		Short: "display Tilt/k8s status at a glance",
	}

	cmd.Flags().StringVar(&c.traceTags, "traceTags", "", "tags to add to spans for easy querying, of the form: key1=val1,key2=val2")

	return cmd
}

func (c *hudCmd) run(ctx context.Context, args []string) error {
	analyticsService.Incr("cmd.hud", nil)
	defer analyticsService.Flush(time.Second)

	span, ctx := opentracing.StartSpanFromContext(ctx, "Up")
	defer span.Finish()

	tags := tracer.TagStrToMap(c.traceTags)
	for k, v := range tags {
		span.SetTag(k, v)
	}

	logOutput(fmt.Sprintf("Starting the HUD (built %s)…\n", buildDateStamp()))

	logOutput("Look I made a HUD!")

	return nil
}
