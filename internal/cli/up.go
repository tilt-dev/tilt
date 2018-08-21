package cli

import (
	"context"
	"errors"
	"log"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/proto"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_client"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_server"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type upCmd struct {
	watch bool
}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <servicename>",
		Short: "stand up a service",
		Args:  cobra.ExactArgs(1),
	}

	cmd.Flags().BoolVar(&c.watch, "watch", false, "any started services will be automatically rebuilt and redeployed when files in their repos change")

	return cmd
}

func (c *upCmd) run(args []string) error {
	span := opentracing.StartSpan("Up")
	defer span.Finish()
	ctx := opentracing.ContextWithSpan(context.Background(), span)
	proc, err := tiltd_server.RunDaemon(ctx)
	if err != nil {
		return err
	}

	defer func() {
		err := proc.Kill()
		if err != nil {
			log.Fatalf("failed to shut down daemon: %v", err)
		}
	}()

	dCli, err := tiltd_client.NewDaemonClient()
	if err != nil {
		return err
	}

	logOutput("Starting Tilt…")

	tf, err := tiltfile.Load("Tiltfile")
	if err != nil {
		return err
	}

	serviceName := args[0]
	services, err := tf.GetServiceConfig(serviceName)
	if err != nil {
		return err
	}

	req := proto.CreateServiceRequest{Services: services, Watch: c.watch, LogLevel: proto.LogLevel(logLevel())}
	err = dCli.CreateService(ctx, req)
	s, ok := status.FromError(err)
	if ok && s.Code() == codes.Unknown {
		return errors.New(s.Message())
	}

	logOutput("Services created")

	return nil
}

func logOutput(s string) {
	cGreen := "\033[32m"
	cReset := "\u001b[0m"
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	log.Printf("%s%s%s", cGreen, s, cReset)
}
