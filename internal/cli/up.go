package cli

import (
	"context"
	"errors"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/proto"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_client"
	"github.com/windmilleng/tilt/internal/tiltd/tiltd_server"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
)

type upCmd struct{}

func (c *upCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <servicename>",
		Short: "stand up a service",
		Args:  cobra.ExactArgs(1),
	}

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

	protoBug := &proto.Debug{Mode: debug}
	err = dCli.SetDebug(ctx, *protoBug)

	tf, err := tiltfile.Load("Tiltfile")
	if err != nil {
		return err
	}

	serviceName := args[0]
	service, err := tf.GetServiceConfig(serviceName)
	if err != nil {
		return err
	}

	err = dCli.CreateService(ctx, *service)
	s, ok := status.FromError(err)
	if ok && s.Code() == codes.Unknown {
		return errors.New(s.Message())
	}

	return nil
}
