package synclet

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s"

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
	name, err := reference.ParseNamed(req.ImageId)
	if err != nil {
		return nil, err
	}

	ref, ok := name.(reference.NamedTagged)
	if !ok {
		return nil, fmt.Errorf("Expected a tagged ref: %s", req.ImageId)
	}

	containerID, err := s.del.ContainerIDForPod(ctx, k8s.PodID(req.PodId), ref)

	if err != nil {
		return nil, err
	}

	return &proto.GetContainerIdForPodReply{ContainerId: string(containerID)}, nil
}

func (s *GRPCServer) UpdateContainer(ctx context.Context, req *proto.UpdateContainerRequest) (*proto.UpdateContainerReply, error) {
	var commands []model.Cmd
	for _, cmd := range req.Commands {
		commands = append(commands, model.Cmd{Argv: cmd.Argv})
	}
	return &proto.UpdateContainerReply{}, s.del.UpdateContainer(ctx, k8s.ContainerID(req.ContainerId), req.TarArchive, req.FilesToDelete, commands)
}
