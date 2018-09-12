package synclet

import (
	"context"

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

func (s *GRPCServer) GetContainerIdForPod(ctx context.Context, req *proto.GetContainerIdForPodRequest) (*proto.GetContainerIdForPodReply, error) {
	containerId, err := s.del.GetContainerIdForPod(ctx, req.PodId)

	if err != nil {
		return nil, err
	}

	return &proto.GetContainerIdForPodReply{ContainerId: containerId}, nil
}

func (s *GRPCServer) UpdateContainer(ctx context.Context, req *proto.UpdateContainerRequest) (*proto.UpdateContainerReply, error) {
	var commands []model.Cmd
	for _, cmd := range req.Commands {
		commands = append(commands, model.Cmd{Argv: cmd.Argv})
	}
	return &proto.UpdateContainerReply{}, s.del.UpdateContainer(ctx, req.ContainerId, req.TarArchive, req.FilesToDelete, commands)
}
