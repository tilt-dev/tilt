package proto

import (
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type grpcServer struct {
	delegate model.ServiceCreator
}

func NewGRPCServer(delegate model.ServiceCreator) *grpcServer {
	return &grpcServer{delegate: delegate}
}

var _ DaemonServer = &grpcServer{}

func (s *grpcServer) CreateService(req *CreateServiceRequest, d Daemon_CreateServiceServer) error {
	sendOutput := func(output Output) error {
		return d.Send(&CreateServiceReply{Output: &output})
	}
	outputStream := MakeStdoutStderrWriter(sendOutput)

	ctx := logger.WithLogger(d.Context(), logger.NewLogger(logger.Level(req.LogLevel), outputStream.stdout))

	var svcArray []model.Service
	for i := range req.Services {
		svcArray = append(svcArray, ServiceP2D(req.Services[i]))
	}

	return s.delegate.CreateServices(ctx, svcArray, req.Watch)
}

func mountsP2D(mounts []*Mount) []model.Mount {
	r := []model.Mount{}

	for _, m := range mounts {
		r = append(r, mountP2D(m))
	}

	return r
}

func mountP2D(mount *Mount) model.Mount {
	return model.Mount{
		Repo:          repoP2D(mount.Repo),
		ContainerPath: mount.ContainerPath,
	}
}

// TODO(dmiller): right now this only supports github repos
// if we add other types we'll have to change this
func repoP2D(repo *Repo) model.LocalGithubRepo {
	githubRepo := repo.GetGitRepo()
	return model.LocalGithubRepo{
		LocalPath: githubRepo.LocalPath,
	}
}

func cmdsP2D(cmds []*Cmd) []model.Cmd {
	r := []model.Cmd{}

	for _, c := range cmds {
		r = append(r, cmdP2D(c))
	}

	return r
}

func cmdP2D(cmd *Cmd) model.Cmd {
	return model.Cmd{
		Argv: cmd.Argv,
	}
}

func serviceNameP2D(s string) model.ServiceName {
	return model.ServiceName(s)
}

// NOTE(maia): public b/c we're hacking out the daemon and will likely put it back soon.
func ServiceP2D(service *Service) model.Service {
	return model.Service{
		K8sYaml:        service.K8SYaml,
		DockerfileText: service.DockerfileText,
		Mounts:         mountsP2D(service.Mounts),
		Steps:          cmdsP2D(service.Steps),
		Entrypoint:     cmdP2D(service.Entrypoint),
		DockerfileTag:  service.DockerfileTag,
		Name:           serviceNameP2D(service.Name),
	}
}
