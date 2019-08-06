package synclet

import (
	"context"
	"sync"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/proto"
)

type syncletDelegate interface {
	UpdateContainer(ctx context.Context, containerID container.ID, tarArchive []byte,
		filesToDelete []string, commands []model.Cmd, hotReload bool) error
}

type GRPCServer struct {
	del syncletDelegate
}

func NewGRPCServer(del syncletDelegate) *GRPCServer {
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

	ctx, err := makeContext(server.Context(), req.LogStyle, send)
	if err != nil {
		return err
	}

	return s.del.UpdateContainer(ctx, container.ID(req.ContainerId), req.TarArchive, req.FilesToDelete, commands, req.HotReload)
}
