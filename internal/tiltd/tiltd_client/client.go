package tiltd_client

import (
	"fmt"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/windmilleng/tilt/internal/proto"
	"github.com/windmilleng/tilt/internal/tiltd"
)

func NewDaemonClient() (*proto.Client, error) {
	conn, err := grpc.Dial(
		fmt.Sprintf("localhost:%d", tiltd.Port),
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
		),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %v", err)
	}

	return proto.NewGRPCClient(conn), nil
}
