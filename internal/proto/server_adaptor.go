package proto

import (
	"github.com/windmilleng/tilt/internal/debug"
	"github.com/windmilleng/tilt/internal/engine"
	"github.com/windmilleng/tilt/internal/model"
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

func (s *GRPCServer) CreateService(req *CreateServiceRequest, d Daemon_CreateServiceServer) error {
	sendOutput := func(output Output) error {
		return d.Send(&CreateServiceReply{Output: &output})
	}

	outputStream := MakeStdoutStderrWriter(sendOutput)

	err := engine.UpService(d.Context(), serviceP2D(req.Service), outputStream.stdout, outputStream.stderr)

	return err
}

func (s *GRPCServer) SetDebug(ctx context.Context, d *Debug) (*DebugReply, error) {
	debug.SetDebugMode(d.Mode)
	return &DebugReply{}, nil
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

func serviceP2D(service *Service) model.Service {
	return model.Service{
		K8sYaml:        service.K8SYaml,
		DockerfileText: service.DockerfileText,
		Mounts:         mountsP2D(service.Mounts),
		Steps:          cmdsP2D(service.Steps),
		Entrypoint:     cmdP2D(service.Entrypoint),
		DockerfileTag:  service.DockerfileTag,
		Name:           service.Name,
	}
}
