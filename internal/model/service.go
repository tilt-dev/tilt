package model

import (
	"context"
	"fmt"
	"strings"
)

type ServiceName string

func (s ServiceName) String() string { return string(s) }

type Service struct {
	K8sYaml        string
	DockerfileText string
	Mounts         []Mount
	Steps          []Cmd
	Entrypoint     Cmd
	DockerfileTag  string
	Name           ServiceName
}

type ServiceCreator interface {
	CreateServices(ctx context.Context, svcs []Service, watch bool) error
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
	quoted := make([]string, len(c.Argv))
	for i, arg := range c.Argv {
		quoted[i] = fmt.Sprintf("%q", arg)
	}
	return fmt.Sprintf("ENTRYPOINT [%s]", strings.Join(quoted, ", "))
}

func (c Cmd) RunStr() string {
	quoted := make([]string, len(c.Argv))
	for i, arg := range c.Argv {
		quoted[i] = fmt.Sprintf("%q", arg)
	}
	return fmt.Sprintf("RUN [%s]", strings.Join(quoted, ", "))
}

func (c Cmd) Empty() bool {
	return len(c.Argv) == 0
}

func ToShellCmd(cmd string) Cmd {
	return Cmd{Argv: []string{"sh", "-c", cmd}}
}

func ToShellCmds(cmds []string) []Cmd {
	res := make([]Cmd, len(cmds))
	for i, cmd := range cmds {
		res[i] = ToShellCmd(cmd)
	}
	return res
}
