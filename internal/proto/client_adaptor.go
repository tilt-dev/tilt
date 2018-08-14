package proto

import (
	"context"

	"google.golang.org/grpc"
	"io"
)

type Client struct {
	del  DaemonClient
	conn *grpc.ClientConn
}

func NewGRPCClient(conn *grpc.ClientConn) *Client {
	return &Client{del: NewDaemonClient(conn), conn: conn}
}

func (c *Client) CreateService(ctx context.Context, service Service) error {
	stream, err := c.del.CreateService(ctx, &service)
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

func (c *Client) SetDebug(ctx context.Context, debug Debug) error {
	_, err := c.del.SetDebug(ctx, &debug)
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
