package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
)

type ManifestName string

func (m ManifestName) String() string { return string(m) }

type Manifest struct {
	K8sYaml        string
	DockerfileText string
	Mounts         []Mount
	Steps          []Step
	Entrypoint     Cmd
	DockerfileTag  reference.Named
	Name           ManifestName
}

func (m Manifest) Validate() error {
	err := m.validate()
	if err != nil {
		return err
	}
	return nil
}

func (m Manifest) validate() *ValidateErr {
	if m.Name == "" {
		return validateErrf("[validate] manifest missing name: %+v", m)
	}

	if m.DockerfileTag == nil {
		return validateErrf("[validate] manifest %q missing image tag", m.Name)
	}

	if m.K8sYaml == "" {
		return validateErrf("[validate] manifest %q missing YAML file", m.Name)
	}

	if m.Entrypoint.Empty() {
		return validateErrf("[validate] manifest %q missing Entrypoint", m.Name)
	}

	return nil
}

type ManifestCreator interface {
	CreateManifests(ctx context.Context, svcs []Manifest, watch bool) error
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

type Step struct {
	// Required. The command to run in this step.
	Cmd Cmd

	// Optional. If not specified, this step runs on every change.
	// If specified, we only run the Cmd if the trigger matches the changed file.
	Trigger PathMatcher
}

type Cmd struct {
	Argv []string
}

func (c Cmd) IsShellStandardForm() bool {
	return len(c.Argv) == 3 && c.Argv[0] == "sh" && c.Argv[1] == "-c" && !strings.Contains(c.Argv[2], "\n")
}

// Get the script when the shell is in standard form.
// Panics if the command is not in shell standard form.
func (c Cmd) ShellStandardScript() string {
	if !c.IsShellStandardForm() {
		panic(fmt.Sprintf("Not in shell standard form: %+v", c))
	}
	return c.Argv[2]
}

func (c Cmd) EntrypointStr() string {
	if c.IsShellStandardForm() {
		return fmt.Sprintf("ENTRYPOINT %s", c.Argv[2])
	}

	quoted := make([]string, len(c.Argv))
	for i, arg := range c.Argv {
		quoted[i] = fmt.Sprintf("%q", arg)
	}
	return fmt.Sprintf("ENTRYPOINT [%s]", strings.Join(quoted, ", "))
}

func (c Cmd) RunStr() string {
	if c.IsShellStandardForm() {
		return fmt.Sprintf("RUN %s", c.Argv[2])
	}

	quoted := make([]string, len(c.Argv))
	for i, arg := range c.Argv {
		quoted[i] = fmt.Sprintf("%q", arg)
	}
	return fmt.Sprintf("RUN [%s]", strings.Join(quoted, ", "))
}
func (c Cmd) String() string {
	if c.IsShellStandardForm() {
		return c.Argv[2]
	}

	quoted := make([]string, len(c.Argv))
	for i, arg := range c.Argv {
		if strings.Contains(arg, " ") {
			quoted[i] = fmt.Sprintf("%q", arg)
		} else {
			quoted[i] = arg
		}
	}
	return fmt.Sprintf("%s", strings.Join(quoted, " "))
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

func ToStep(cmd Cmd) Step {
	return Step{Cmd: cmd}
}

func ToSteps(cmds []Cmd) []Step {
	res := make([]Step, len(cmds))
	for i, cmd := range cmds {
		res[i] = ToStep(cmd)
	}
	return res
}

func ToShellSteps(cmds []string) []Step {
	return ToSteps(ToShellCmds(cmds))
}

type ValidateErr struct {
	s string
}

func (e *ValidateErr) Error() string { return e.s }

var _ error = &ValidateErr{}

func validateErrf(format string, a ...interface{}) *ValidateErr {
	return &ValidateErr{s: fmt.Sprintf(format, a...)}
}
