package synclet

import (
	"context"
	"io"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/build"

	"github.com/windmilleng/tilt/internal/container"

	"github.com/windmilleng/tilt/pkg/logger"

	"github.com/windmilleng/tilt/internal/synclet/proto"
	"github.com/windmilleng/tilt/pkg/model"

	"google.golang.org/grpc"
)

type SyncletClient interface {
	UpdateContainer(ctx context.Context, containerID container.ID, tarArchive []byte,
		filesToDelete []string, commands []model.Cmd, hotReload bool) error

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
	commands []model.Cmd,
	hotReload bool) error {

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
		HotReload:     hotReload,
	})

	if err != nil {
		return errors.Wrap(err, "failed invoking synclet.UpdateContainer")
	}

	var runStepFailure build.RunStepFailure
	for {
		reply, err := stream.Recv()

		if reply != nil && reply.FailedRunStep != nil {
			frs := reply.FailedRunStep
			runStepFailure = build.RunStepFailure{
				Cmd:      model.Cmd{Argv: []string{frs.Cmd}},
				ExitCode: int(frs.ExitCode),
			}
			continue
		}

		if err == io.EOF {
			if !runStepFailure.Empty() {
				return runStepFailure
			}
			return nil
		} else if err != nil {
			return errors.Wrap(err, "error from synclet.UpdateContainer")
		}

		level := protoLogLevelToLevel(reply.LogMessage.Level)

		logger.Get(ctx).Write(level, reply.LogMessage.Message)
	}
}

func (s *SyncletCli) Close() error {
	return s.conn.Close()
}
