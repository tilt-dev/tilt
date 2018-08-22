package cli

import (
	"fmt"
	"log"
	"net"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/k8s"
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
	env, err := k8s.DetectEnv()
	if err != nil {
		return fmt.Errorf("failed to detect kubernetes: %v", err)
	}

	serviceManager := service.NewMemoryManager()
	upperCreator := engine.NewUpperServiceCreator(serviceManager, env)
	creator := service.TrackServices(upperCreator, serviceManager)

	proto.RegisterDaemonServer(s, proto.NewGRPCServer(creator))

	err = s.Serve(l)
	if err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}
	return nil
}
