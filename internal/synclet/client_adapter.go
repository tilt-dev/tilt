package synclet

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/windmilleng/tilt/internal/logger"

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

func protoLogLevelToLevel(protoLevel proto.LogLevel) (logger.Level, error) {
	var level logger.Level
	switch protoLevel {
	case proto.LogLevel_INFO:
		level = logger.InfoLvl
	case proto.LogLevel_VERBOSE:
		level = logger.VerboseLvl
	case proto.LogLevel_DEBUG:
		level = logger.DebugLvl
	default:
		return logger.InfoLvl, fmt.Errorf("unknown log level '%v'", protoLevel)
	}

	return level, nil
}

func newProtoContext(ctx context.Context) *proto.Context {
	return &proto.Context{LogColorsEnabled: logger.Get(ctx).SupportsColor()}
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

	stream, err := s.del.UpdateContainer(ctx, &proto.UpdateContainerRequest{
		Context:       newProtoContext(ctx),
		ContainerId:   containerId.String(),
		TarArchive:    tarArchive,
		FilesToDelete: filesToDelete,
		Commands:      protoCmds,
	})

	if err != nil {
		return err
	}

	for {
		reply, err := stream.Recv()

		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		level, err := protoLogLevelToLevel(reply.LogMessage.Level)
		if err != nil {
			return err
		}

		logger.Get(ctx).Write(level, string(reply.LogMessage.Message))
	}

	return err
}

func (s *SyncletCli) ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (k8s.ContainerID, error) {
	stream, err := s.del.GetContainerIdForPod(ctx, &proto.GetContainerIdForPodRequest{
		Context: newProtoContext(ctx),
		PodId:   podID.String(),
		ImageId: imageID.String(),
	})
	if err != nil {
		return "", err
	}

	for {
		reply, err := stream.Recv()

		if err == io.EOF {
			return k8s.ContainerID(""), errors.New("internal error: GetContainerIdForPod reached eof without returning either an error or a container id")
		} else if err != nil {
			return k8s.ContainerID(""), err
		}

		switch x := reply.Content.(type) {
		case *proto.GetContainerIdForPodReply_ContainerId:
			return k8s.ContainerID(x.ContainerId), nil
		case *proto.GetContainerIdForPodReply_Message:
			level, err := protoLogLevelToLevel(x.Message.Level)
			if err != nil {
				return k8s.ContainerID(""), err
			}

			logger.Get(ctx).Write(level, string(x.Message.Message))
		}
	}
}

func (s *SyncletCli) Close() error {
	return s.conn.Close()
}
