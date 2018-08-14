package proto

import (
	"github.com/windmilleng/tilt/internal/build"
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
	return &CreateServiceReply{}, s.del.CreateService(ctx, service.K8SYaml, service.DockerfileText, mountsP2D(service.Mounts), cmdsP2D(service.Steps), cmdP2D(service.Entrypoint), service.DockerfileTag)
}

func mountsP2D(mounts []*Mount) []build.Mount {
	r := []build.Mount{}

	for _, m := range mounts {
		r = append(r, mountP2D(m))
	}

	return r
}

func mountP2D(mount *Mount) build.Mount {
	return build.Mount{
		Repo:          repoP2D(mount.Repo),
		ContainerPath: mount.ContainerPath,
	}
}

// TODO(dmiller): right now this only supports github repos
// if we add other types we'll have to change this
func repoP2D(repo *Repo) build.LocalGithubRepo {
	githubRepo := repo.GetGitRepo()
	return build.LocalGithubRepo{
		LocalPath: githubRepo.LocalPath,
	}
}

func cmdsP2D(cmds []*Cmd) []build.Cmd {
	r := []build.Cmd{}

	for _, c := range cmds {
		r = append(r, cmdP2D(c))
	}

	return r
}

func cmdP2D(cmd *Cmd) build.Cmd {
	return build.Cmd{
		Argv: cmd.Argv,
	}
}
