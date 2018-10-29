package synclet

import (
	"context"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/container"

	"github.com/windmilleng/tilt/internal/logger"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/proto"

	"google.golang.org/grpc"
)

const containerIdTimeout = time.Second * 10
const containerIdRetryDelay = time.Millisecond * 100

type SyncletClient interface {
	UpdateContainer(ctx context.Context, containerID container.ID, tarArchive []byte,
		filesToDelete []string, commands []model.Cmd) error
	ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (container.ID, error)

	Close() error
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
	containerId container.ID,
	tarArchive []byte,
	filesToDelete []string,
	commands []model.Cmd) error {

	var protoCmds []*proto.Cmd

	for _, cmd := range commands {
		protoCmds = append(protoCmds, &proto.Cmd{Argv: cmd.Argv})
	}

	logStyle, err := newLogStyle(ctx)
	if err != nil {
		return err
	}

	stream, err := s.del.UpdateContainer(ctx, &proto.UpdateContainerRequest{
		LogStyle:      logStyle,
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

func (s *SyncletCli) ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (cID container.ID, err error) {
	timeout := time.After(containerIdTimeout)
	for {
		// TODO(maia): better distinction between errs meaning "couldn't connect yet"
		// and "everything is borked, stop trying"
		cID, err = s.containerIDForPod(ctx, podID, imageID)
		if !cID.Empty() {
			return cID, nil
		}

		retryTimer := time.NewTimer(containerIdRetryDelay)

		select {
		case <-timeout:
			return "", errors.Wrapf(err, "timed out trying to get container ID for pod %s (after %s). Latest err",
				podID.String(), containerIdTimeout)
		case <-ctx.Done():
			return "", errors.New("ctx was cancelled")
		case <-retryTimer.C:
		}
	}
}

func (s *SyncletCli) containerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (container.ID, error) {
	logStyle, err := newLogStyle(ctx)
	if err != nil {
		return "", err
	}

	stream, err := s.del.GetContainerIdForPod(ctx, &proto.GetContainerIdForPodRequest{
		LogStyle: logStyle,
		PodId:    podID.String(),
		ImageId:  imageID.String(),
	})
	if err != nil {
		return "", err
	}

	for {
		reply, err := stream.Recv()

		if err == io.EOF {
			return container.ID(""), errors.New("internal error: GetContainerIdForPod reached eof without returning either an error or a container id")
		} else if err != nil {
			return container.ID(""), errors.Wrap(err, "error returned from synclet.GetContainerIdForPod")
		}

		switch x := reply.Content.(type) {
		case *proto.GetContainerIdForPodReply_ContainerId:
			return container.ID(x.ContainerId), nil
		case *proto.GetContainerIdForPodReply_Message:
			level := protoLogLevelToLevel(x.Message.Level)

			logger.Get(ctx).Write(level, string(x.Message.Message))
		}
	}
}

func (s *SyncletCli) Close() error {
	return s.conn.Close()
}
