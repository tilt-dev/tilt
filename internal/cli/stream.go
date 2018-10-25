package cli

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/stream"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
)

type streamCmd struct {
}

func (c *streamCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "display streaming tilt output",
	}

	return cmd
}

// blocks until we have a successful connection established
func connect(ctx context.Context) (<-chan stream.MessageOrError, func() error, error) {
	cli := stream.NewStreamClient()
	ch, err := cli.Connect(ctx)

	if err != nil {
		log.Printf("Waiting for `tilt up` to start.")

		for {
			cli := stream.NewStreamClient()
			ch, err = cli.Connect(ctx)
			if err == nil {
				break
			}
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}

	return ch, cli.Close, err
}

func (c *streamCmd) run(ctx context.Context, args []string) error {
	analyticsService.Incr("cmd.stream", nil)
	defer analyticsService.Flush(time.Second)

	span, ctx := opentracing.StartSpanFromContext(ctx, "stream")
	defer span.Finish()

	for {
		ch, closer, err := connect(ctx)
		if err != nil {
			if err == context.Canceled {
				return nil
			}
			return errors.Wrap(err, "error connecting to stream server")
		}
		defer func() {
			err = closer()
			if err != nil {
				fmt.Printf("error closing stream client: %v", err)
			}
		}()

		err = processConnection(ctx, ch)
		if err != nil {
			if err == context.Canceled {
				return nil
			}
			return errors.Wrap(err, "error handling tilt stream")
		}

		log.Printf("Connection to tilt completed.\n")
	}
}

func processConnection(ctx context.Context, ch <-chan stream.MessageOrError) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msgOrError, ok := <-ch:
			if !ok {
				return nil
			} else if msgOrError.Err != nil {
				return msgOrError.Err
			} else {
				fmt.Print(msgOrError.Message)
			}
		}
	}
}
