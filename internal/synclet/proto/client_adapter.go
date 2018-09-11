package proto

import (
	"context"
	"github.com/windmilleng/tilt/internal/model"

	"google.golang.org/grpc"
)

type Client struct {
	del  SyncletClient
	conn *grpc.ClientConn
}

func NewGRPCClient(conn *grpc.ClientConn) *Client {
	return &Client{del: NewSyncletClient(conn), conn: conn}
}

func (c *Client) UpdateContainer(
	ctx context.Context,
	containerId string,
	tarArchive []byte,
	filesToDelete []string,
	commands []model.Cmd) error {

	var protoCmds []*Cmd

	for _, cmd := range commands {
		protoCmds = append(protoCmds, &Cmd{Argv: cmd.Argv})
	}

	_, err := c.del.UpdateContainer(ctx, &UpdateContainerRequest{
		ContainerId: containerId,
		TarArchive: tarArchive,
		FilesToDelete: filesToDelete,
		Commands: protoCmds,
	})

	return err
}

func (c *Client) GetContainerIdForPod(ctx context.Context, podId string) (string, error) {
	reply, err := c.del.GetContainerIdForPod(ctx, &GetContainerIdForPodRequest{PodId: podId})
	if err != nil {
		return "", err
	}

	return reply.ContainerId, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}