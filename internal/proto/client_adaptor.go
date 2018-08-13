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

		body := reply.GetBody()
		if body != nil {
			switch reply.GetBody().(type) {
			case *CreateServiceReply_Output:
				output := reply.GetOutput()
				printOutput(*output)
			}
		}
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}
