package cli

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type upCmd struct {
	watch       bool
	browserMode engine.BrowserMode
	cleanUpFn   func() error
}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <servicename>",
		Short: "stand up a service",
		Args:  cobra.ExactArgs(1),
	}

	cmd.Flags().BoolVar(&c.watch, "watch", false, "any started services will be automatically rebuilt and redeployed when files in their repos change")
	cmd.Flags().Var(&c.browserMode, "browser", "open a browser when the service first starts")

	return cmd
}

func (c *upCmd) run(args []string) error {
	span := opentracing.StartSpan("Up")
	defer span.Finish()
	l := logger.NewLogger(logLevel(), os.Stdout)
	ctx := output.WithOutputter(
		logger.WithLogger(
			opentracing.ContextWithSpan(context.Background(), span),
			l),
		output.NewOutputter(l))

	// SIGNAL TRAPPING
	ctx, cancel := context.WithCancel(ctx)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_ = <-sigs

		// Clean up anything that needs cleaning up
		err := c.cleanUpFn()
		if err != nil {
			l.Infof("error cleaning up: %v", err)
		}

		// We rely on context cancellation being handled elsewhere --
		// otherwise there's no way to SIGINT/SIGTERM this app o_0
		cancel()
	}()

	logOutput("Starting Tilt…")

	tf, err := tiltfile.Load("Tiltfile")
	if err != nil {
		return err
	}

	serviceName := args[0]
	services, err := tf.GetServiceConfigs(serviceName)
	if err != nil {
		return err
	}

	serviceCreator, err := wireServiceCreator(ctx, c.browserMode)
	if err != nil {
		return err
	}

	err = serviceCreator.CreateServices(ctx, services, c.watch)
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

	logOutput("Services created")

	return nil
}

func logOutput(s string) {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	log.Printf(color.GreenString(s))
}
