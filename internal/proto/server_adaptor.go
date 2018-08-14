package proto

import (
	"github.com/windmilleng/tilt/internal/tiltd"
)

type GRPCServer struct {
	del tiltd.TiltD
}

func NewGRPCServer(del tiltd.TiltD) *GRPCServer {
	return &GRPCServer{del: del}
}

var _ DaemonServer = &GRPCServer{}

func (s *GRPCServer) CreateService(service *Service, d Daemon_CreateServiceServer) error {
	sendOutput := func(output Output) error {
		return d.Send(&CreateServiceReply{Output: &output})
	}

	outputStream := MakeStdoutStderrWriter(sendOutput)

	err := s.del.CreateService(
		d.Context(),
		service.K8SYaml,
		service.DockerfileText,
		mountsP2D(service.Mounts),
		cmdsP2D(service.Steps),
		service.DockerfileTag,
		outputStream.stdout,
		outputStream.stderr)

	return err
}

func (s *GRPCServer) SetDebug(ctx context.Context, debug *Debug) (*DebugReply, error) {
	return &DebugReply{}, s.del.SetDebug(ctx, debug.Mode)
}

func mountsP2D(mounts []*Mount) []tiltd.Mount {
	r := []tiltd.Mount{}

	for _, m := range mounts {
		r = append(r, mountP2D(m))
	}

	return r
}

func mountP2D(mount *Mount) tiltd.Mount {
	return tiltd.Mount{
		Repo:          repoP2D(mount.Repo),
		ContainerPath: mount.ContainerPath,
	}
}

// TODO(dmiller): right now this only supports github repos
// if we add other types we'll have to change this
func repoP2D(repo *Repo) tiltd.LocalGithubRepo {
	githubRepo := repo.GetGitRepo()
	return tiltd.LocalGithubRepo{
		LocalPath: githubRepo.LocalPath,
	}
}

func cmdsP2D(cmds []*Cmd) []tiltd.Cmd {
	r := []tiltd.Cmd{}

	for _, c := range cmds {
		r = append(r, cmdP2D(c))
	}

	return r
}

func cmdP2D(cmd *Cmd) tiltd.Cmd {
	return tiltd.Cmd{
		Argv: cmd.Argv,
	}
}
