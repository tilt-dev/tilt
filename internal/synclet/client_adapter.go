package synclet

import (
	"context"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/proto"

	"google.golang.org/grpc"
)

type SyncletClient interface {
	UpdateContainer(ctx context.Context, containerID k8s.ContainerID, tarArchive []byte,
		filesToDelete []string, commands []model.Cmd) error
	ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (k8s.ContainerID, error)
}

var _ SyncletClient = &SyncletCli{}

type SyncletCli struct {
	del  proto.SyncletClient
	conn *grpc.ClientConn
}

func NewGRPCClient(conn *grpc.ClientConn) *SyncletCli {
	return &SyncletCli{del: proto.NewSyncletClient(conn), conn: conn}
}

func (s *SyncletCli) UpdateContainer(
	ctx context.Context,
	containerId k8s.ContainerID,
	tarArchive []byte,
	filesToDelete []string,
	commands []model.Cmd) error {

	var protoCmds []*proto.Cmd

	for _, cmd := range commands {
		protoCmds = append(protoCmds, &proto.Cmd{Argv: cmd.Argv})
	}

	_, err := s.del.UpdateContainer(ctx, &proto.UpdateContainerRequest{
		ContainerId:   containerId.String(),
		TarArchive:    tarArchive,
		FilesToDelete: filesToDelete,
		Commands:      protoCmds,
	})

	return err
}

func (s *SyncletCli) ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (k8s.ContainerID, error) {
	reply, err := s.del.GetContainerIdForPod(ctx, &proto.GetContainerIdForPodRequest{
		PodId:   podID.String(),
		ImageId: imageID.String(),
	})
	if err != nil {
		return "", err
	}

	return k8s.ContainerID(reply.ContainerId), nil
}

func (s *SyncletCli) Close() error {
	return s.conn.Close()
}
