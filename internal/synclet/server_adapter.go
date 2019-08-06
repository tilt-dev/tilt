package synclet

import (
	"sync"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/proto"
)

type GRPCServer struct {
	del *Synclet
}

func NewGRPCServer(del *Synclet) *GRPCServer {
	return &GRPCServer{del: del}
}

var _ proto.SyncletServer = &GRPCServer{}

func (s *GRPCServer) UpdateContainer(req *proto.UpdateContainerRequest, server proto.Synclet_UpdateContainerServer) error {
	var commands []model.Cmd
	for _, cmd := range req.Commands {
		commands = append(commands, model.Cmd{Argv: cmd.Argv})
	}

	sendMutex := new(sync.Mutex)
	send := func(m *proto.LogMessage) error {
		sendMutex.Lock()
		defer sendMutex.Unlock()
		return server.Send(&proto.UpdateContainerReply{LogMessage: m})
	}

	sendRSF := func(rsf build.RunStepFailure) error {
		sendMutex.Lock()
		defer sendMutex.Unlock()
		return server.Send(&proto.UpdateContainerReply{FailedRunStep: &proto.FailedRunStep{
			Cmd:      rsf.Cmd.String(),
			ExitCode: int32(rsf.ExitCode),
		}})
	}

	ctx, err := makeContext(server.Context(), req.LogStyle, send)
	if err != nil {
		return err
	}

	err = s.del.UpdateContainer(ctx, container.ID(req.ContainerId), req.TarArchive, req.FilesToDelete, commands, req.HotReload)
	if rsf, ok := build.MaybeRunStepFailure(err); ok {
		return sendRSF(rsf)
	}
	return err
}
