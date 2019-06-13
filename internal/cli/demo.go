package cli

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/demo"
)

type demoCmd struct {
	branch string
}

func (c *demoCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Run the demo script",
	}

	cmd.Flags().StringVar(&c.branch, "branch", "",
		"Checks out a branch of the tiltdemo repo, instead of the master branch")

	return cmd
}

func (c *demoCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	a.Incr("cmd.demo", map[string]string{})
	defer a.Flush(time.Second)

	demo, err := wireDemo(ctx, demo.RepoBranch(c.branch), a)
	if err != nil {
		return err
	}

	err = demo.Run(ctx)
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
