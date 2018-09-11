package proto

import (
	"context"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet"
)

type GRPCServer struct {
	del *synclet.Synclet
}

func NewGRPCServer(del *synclet.Synclet) *GRPCServer {
	return &GRPCServer{del: del}
}

var _ SyncletServer = &GRPCServer{}

func (s *GRPCServer) GetContainerIdForPod(ctx context.Context, req *GetContainerIdForPodRequest) (*GetContainerIdForPodReply, error) {
	containerId, err := s.del.GetContainerIdForPod(req.PodId)

	if err != nil {
		return nil, err
	}

	return &GetContainerIdForPodReply{ContainerId: containerId}, nil
}

func (s *GRPCServer) UpdateContainer(ctx context.Context, req *UpdateContainerRequest) (*UpdateContainerReply, error) {
	var commands []model.Cmd
	for _, cmd := range req.Commands {
		commands = append(commands, model.Cmd{Argv: cmd.Argv})
	}
	return &UpdateContainerReply{}, s.del.UpdateContainer(ctx, req.ContainerId, req.TarArchive, req.FilesToDelete, commands)
}
