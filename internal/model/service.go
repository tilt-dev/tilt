package model

import (
	"fmt"
	"strings"
)

type ServiceName string

type Service struct {
	K8sYaml        string
	DockerfileText string
	Mounts         []Mount
	Steps          []Cmd
	Entrypoint     Cmd
	DockerfileTag  string
	Name           ServiceName
}

type Mount struct {
	// TODO(dmiller) make this more generic
	// TODO(maia): or maybe don't make this a repo necessarily, just a path?
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
