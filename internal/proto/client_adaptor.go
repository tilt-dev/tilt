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

func (c *Client) CreateService(ctx context.Context, yaml string) error {
	_, err := c.del.CreateService(ctx, &Service{K8SYaml: yaml})
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
