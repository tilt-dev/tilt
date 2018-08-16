package proto

import (
	"context"

	"io"

	"google.golang.org/grpc"
)

type Client struct {
	del  DaemonClient
	conn *grpc.ClientConn
}

func NewGRPCClient(conn *grpc.ClientConn) *Client {
	return &Client{del: NewDaemonClient(conn), conn: conn}
}

func (c *Client) CreateService(ctx context.Context, req CreateServiceRequest) error {
	stream, err := c.del.CreateService(ctx, &req)
	if err != nil {
		return err
	}

	for {
		reply, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		output := reply.GetOutput()
		if output != nil {
			err := printOutput(*reply.Output)
			if err != nil {
				return err
			}
		}
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}
