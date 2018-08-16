package model

import (
	"fmt"
	"strings"
)

type Service struct {
	K8sYaml        string
	DockerfileText string
	Mounts         []Mount
	Steps          []Cmd
	Entrypoint     Cmd
	DockerfileTag  string
	Name           string
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

func (c Cmd) Empty() bool {
	return len(c.Argv) == 0
}

func ToShellCmd(cmd string) Cmd {
	return Cmd{Argv: []string{"sh", "-c", cmd}}
}
