package tiltd

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const Port = 10000

type TiltD interface {
	CreateService(ctx context.Context, k8sYaml string, dockerFileText string, mounts []Mount, steps []Cmd, dockerfileTag string, stdout io.Writer, stderr io.Writer) error
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
