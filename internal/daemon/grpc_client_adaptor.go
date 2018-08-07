package daemon

import (
	"context"

	"github.com/windmilleng/tilt/internal/proto"
	"google.golang.org/grpc"
)

type Client struct {
	del  proto.DaemonClient
	conn *grpc.ClientConn
}

func NewGRPCClient(conn *grpc.ClientConn) *Client {
	return &Client{del: proto.NewDaemonClient(conn), conn: conn}
}

func (c *Client) CreateService(ctx context.Context, yaml string) error {
	_, err := c.del.CreateService(ctx, &proto.Service{K8SYaml: yaml})
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
