package tiltd

import (
	"context"

	"github.com/windmilleng/tilt/internal/proto"
)

type GRPCServer struct {
	del *Daemon
}

func NewGRPCServer(del *Daemon) *GRPCServer {
	return &GRPCServer{del: del}
}

var _ proto.DaemonServer = &GRPCServer{}

func (s *GRPCServer) CreateService(ctx context.Context, service *proto.Service) (*proto.CreateServiceReply, error) {
	return &proto.CreateServiceReply{}, s.del.CreateService(ctx, service.K8SYaml)
}
