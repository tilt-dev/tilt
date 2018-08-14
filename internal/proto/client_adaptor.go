package proto

import (
	"context"

	"google.golang.org/grpc"
)

type Client struct {
	del  DaemonClient
	conn *grpc.ClientConn
}

func NewGRPCClient(conn *grpc.ClientConn) *Client {
	return &Client{del: NewDaemonClient(conn), conn: conn}
}

func (c *Client) CreateService(ctx context.Context, service Service) error {
	_, err := c.del.CreateService(ctx, &service)
	return err
}

func (c *Client) SetDebug(ctx context.Context, debug Debug) error {
	_, err := c.del.SetDebug(ctx, &debug)
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
