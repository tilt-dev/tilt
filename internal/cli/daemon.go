package cli

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/proto"
	"github.com/windmilleng/tilt/internal/service"
	"github.com/windmilleng/tilt/internal/tiltd"
	"github.com/windmilleng/tilt/internal/tracer"
)

type daemonCmd struct{}

func (c *daemonCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "daemon",
		Short:  "start the daemon",
		Hidden: true,
	}

	return cmd
}

func (c *daemonCmd) run(args []string) error {
	err := tracer.Init()
	if err != nil {
		log.Printf("Warning: unable to initialize tracer: %s", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", tiltd.Port)
	log.Printf("Running tiltd listening on %s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(
			otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer())),
		grpc.StreamInterceptor(
			otgrpc.OpenTracingStreamServerInterceptor(opentracing.GlobalTracer())),
	)

	ctx := context.Background()
	serviceCreator, err := wireServiceCreator(ctx, engine.BrowserAuto)
	if err != nil {
		return fmt.Errorf("failed to build daemon: %v", err)
	}
	proto.RegisterDaemonServer(s, proto.NewGRPCServer(serviceCreator))

	err = s.Serve(l)
	if err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}
	return nil
}

func provideServiceCreator(upper engine.Upper, sm service.Manager) model.ServiceCreator {
	return service.TrackServices(upper, sm)
}
