package cli

import (
	"context"
	"errors"
	"log"

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
	ctx := context.Background()
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

	tf, err := tiltfile.Load("Tiltfile")
	if err != nil {
		return err
	}

	serviceName := args[0]
	service, err := tf.GetServiceConfig(serviceName)
	if err != nil {
		return err
	}

	for i := range service {
		var req []proto.CreateServiceRequest
		req[i] = proto.CreateServiceRequest{Service: service[i], Watch: c.watch, LogLevel: proto.LogLevel(logLevel())}
		err = dCli.CreateService(ctx, req[i])
		s, ok := status.FromError(err)
		if ok && s.Code() == codes.Unknown {
			return errors.New(s.Message())
		}
	}

	return nil
}
