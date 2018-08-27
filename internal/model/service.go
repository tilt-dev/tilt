package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/mitchellh/hashstructure"
)

type ServiceName string

func (s ServiceName) String() string { return string(s) }

type Service struct {
	K8sYaml        string `hash:"ignore"`
	DockerfileText string
	Mounts         []Mount
	Steps          []Cmd
	Entrypoint     Cmd
	DockerfileTag  string      `hash:"ignore"`
	Name           ServiceName `hash:"ignore"`
}

type HashedService = uint64

func (s *Service) Hash() (HashedService, error) {
	hash, err := hashstructure.Hash(s, nil)
	if err != nil {
		return 0, err
	}

	return HashedService(hash), nil
}

func (s Service) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("Service missing name: %+v", s)
	}

	if s.DockerfileTag == "" {
		return fmt.Errorf("Service %q missing image tag", s.Name)
	}

	if s.K8sYaml == "" {
		return fmt.Errorf("Service %q missing YAML file", s.Name)
	}

	if s.Entrypoint.Empty() {
		return fmt.Errorf("Service %q missing Entrypoint", s.Name)
	}

	return nil
}

type ServiceCreator interface {
	CreateServices(ctx context.Context, svcs []Service, watch bool, dryrun bool) error
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

func (c Cmd) isShellStandardForm() bool {
	return len(c.Argv) == 3 && c.Argv[0] == "sh" && c.Argv[1] == "-c" && !strings.Contains(c.Argv[2], "\n")
}

func (c Cmd) EntrypointStr() string {
	if c.isShellStandardForm() {
		return fmt.Sprintf("ENTRYPOINT %s", c.Argv[2])
	}

	quoted := make([]string, len(c.Argv))
	for i, arg := range c.Argv {
		quoted[i] = fmt.Sprintf("%q", arg)
	}
	return fmt.Sprintf("ENTRYPOINT [%s]", strings.Join(quoted, ", "))
}

func (c Cmd) RunStr() string {
	if c.isShellStandardForm() {
		return fmt.Sprintf("RUN %s", c.Argv[2])
	}

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
