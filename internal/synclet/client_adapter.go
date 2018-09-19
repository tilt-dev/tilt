package synclet

import (
	"context"
	"io"

	"github.com/pkg/errors"

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

func protoLogLevelToLevel(protoLevel proto.LogLevel) logger.Level {
	switch protoLevel {
	case proto.LogLevel_INFO:
		return logger.InfoLvl
	case proto.LogLevel_VERBOSE:
		return logger.VerboseLvl
	case proto.LogLevel_DEBUG:
		return logger.DebugLvl
	default:
		// the server returned a log level that we don't recognize - err on the side of caution and return
		// the minimum log level to ensure that all output is printed
		return logger.NoneLvl
	}
}

func newLogStyle(ctx context.Context) *proto.LogStyle {
	return &proto.LogStyle{ColorsEnabled: logger.Get(ctx).SupportsColor()}
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
		LogStyle:      newLogStyle(ctx),
		ContainerId:   containerId.String(),
		TarArchive:    tarArchive,
		FilesToDelete: filesToDelete,
		Commands:      protoCmds,
	})

	if err != nil {
		return errors.Wrap(err, "failed invoking synclet.UpdateContainer")
	}

	for {
		reply, err := stream.Recv()

		if err == io.EOF {
			return nil
		} else if err != nil {
			return errors.Wrap(err, "error from synclet.UpdateContainer")
		}

		level := protoLogLevelToLevel(reply.LogMessage.Level)

		logger.Get(ctx).Write(level, string(reply.LogMessage.Message))
	}
}

func (s *SyncletCli) ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (k8s.ContainerID, error) {
	stream, err := s.del.GetContainerIdForPod(ctx, &proto.GetContainerIdForPodRequest{
		LogStyle: newLogStyle(ctx),
		PodId:    podID.String(),
		ImageId:  imageID.String(),
	})
	if err != nil {
		return "", err
	}

	for {
		reply, err := stream.Recv()

		if err == io.EOF {
			return k8s.ContainerID(""), errors.New("internal error: GetContainerIdForPod reached eof without returning either an error or a container id")
		} else if err != nil {
			return k8s.ContainerID(""), errors.Wrap(err, "error returned from synclet.GetContainerIdForPod")
		}

		switch x := reply.Content.(type) {
		case *proto.GetContainerIdForPodReply_ContainerId:
			return k8s.ContainerID(x.ContainerId), nil
		case *proto.GetContainerIdForPodReply_Message:
			level := protoLogLevelToLevel(x.Message.Level)

			logger.Get(ctx).Write(level, string(x.Message.Message))
		}
	}
}

func (s *SyncletCli) Close() error {
	return s.conn.Close()
}
