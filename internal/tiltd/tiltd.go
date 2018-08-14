package tiltd

import (
	"context"
	"fmt"
	"strings"
)

const Port = 10000

type TiltD interface {
	CreateService(ctx context.Context, k8sYaml string, dockerFileText string, mounts []Mount, steps []Cmd, dockerfileTag string) error
	SetDebug(ctx context.Context, mode bool)
}

type Debug struct {
	Mode bool
}

type Mount struct {
	// TODO(dmiller) make this more generic
	Repo          LocalGithubRepo
	ContainerPath string
}

type Repo interface {
	IsRepo()
}

type LocalGithubRepo struct {
	LocalPath string
}

func (LocalGithubRepo) IsRepo() {}

type Cmd struct {
	Argv []string
}

func (c Cmd) EntrypointStr() string {
	return fmt.Sprintf("ENTRYPOINT [\"%s\"]", strings.Join(c.Argv, "\", \""))
}
