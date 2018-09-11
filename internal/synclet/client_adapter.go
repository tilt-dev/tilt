package synclet

import (
	"context"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/proto"

	"google.golang.org/grpc"
)

type Client struct {
	del  proto.SyncletClient
	conn *grpc.ClientConn
}

func NewGRPCClient(conn *grpc.ClientConn) *Client {
	return &Client{del: proto.NewSyncletClient(conn), conn: conn}
}

func (c *Client) UpdateContainer(
	ctx context.Context,
	containerId string,
	tarArchive []byte,
	filesToDelete []string,
	commands []model.Cmd) error {

	var protoCmds []*proto.Cmd

	for _, cmd := range commands {
		protoCmds = append(protoCmds, &proto.Cmd{Argv: cmd.Argv})
	}

	_, err := c.del.UpdateContainer(ctx, &proto.UpdateContainerRequest{
		ContainerId:   containerId,
		TarArchive:    tarArchive,
		FilesToDelete: filesToDelete,
		Commands:      protoCmds,
	})

	return err
}

func (c *Client) GetContainerIdForPod(ctx context.Context, podId k8s.PodID) (k8s.ContainerID, error) {
	reply, err := c.del.GetContainerIdForPod(ctx, &proto.GetContainerIdForPodRequest{PodId: podId.String()})
	if err != nil {
		return "", err
	}

	return k8s.ContainerID(reply.ContainerId), nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
