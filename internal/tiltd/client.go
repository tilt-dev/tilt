package tiltd

import (
	"fmt"

	"google.golang.org/grpc"
)

func NewDaemonClient() (*Client, error) {
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", Port),
		grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %v", err)
	}

	return NewGRPCClient(conn), nil
}
