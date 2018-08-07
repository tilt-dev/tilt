package tiltd_client

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/proto"
	"github.com/windmilleng/tilt/internal/tiltd"
	"google.golang.org/grpc"
)

func NewDaemonClient() (*proto.Client, error) {
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", tiltd.Port),
		grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %v", err)
	}

	return proto.NewGRPCClient(conn), nil
}
