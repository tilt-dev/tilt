package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/proto"
	"github.com/windmilleng/tilt/internal/service"
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
	ctx := logger.WithLogger(
		opentracing.ContextWithSpan(context.Background(), span),
		logger.NewLogger(logLevel(), os.Stdout),
	)

	logOutput("Starting Tilt…")

	tf, err := tiltfile.Load("Tiltfile")
	if err != nil {
		return err
	}

	serviceName := args[0]
	protoServices, err := tf.GetServiceConfig(serviceName)
	if err != nil {
		return err
	}

	services := make([]model.Service, len(protoServices))
	for i, s := range protoServices {
		services[i] = proto.ServiceP2D(s)
	}

	env, err := k8s.DetectEnv()
	if err != nil {
		return fmt.Errorf("failed to detect kubernetes: %v", err)
	}

	serviceManager := service.NewMemoryManager()
	upperCreator := engine.NewUpperServiceCreator(serviceManager, env)
	creator := service.TrackServices(upperCreator, serviceManager)

	if err != nil {
		return err
	}

	err = creator.CreateServices(ctx, services, c.watch)
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
