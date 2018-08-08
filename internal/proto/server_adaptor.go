package proto

import (
	"github.com/windmilleng/tilt/internal/tiltd"
	context "golang.org/x/net/context"
)

type GRPCServer struct {
	del tiltd.TiltD
}

func NewGRPCServer(del tiltd.TiltD) *GRPCServer {
	return &GRPCServer{del: del}
}

var _ DaemonServer = &GRPCServer{}

func (s *GRPCServer) CreateService(ctx context.Context, service *Service) (*CreateServiceReply, error) {
	return &CreateServiceReply{}, s.del.CreateService(ctx, service.K8SYaml)
}
