package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
)

type ManifestName string

func (m ManifestName) String() string { return string(m) }

// NOTE: If you modify Manifest, make sure to modify `Manifest.Equal` appropriately
type Manifest struct {
	// Properties for all builds.
	Name         ManifestName
	K8sYaml      string
	TiltFilename string
	DockerRef    reference.Named

	// Local files read while reading the Tilt configuration.
	// If these files are changed, we should reload the manifest.
	ConfigFiles []string

	// Properties for fast_build (builds that support
	// iteration based on past artifacts)
	BaseDockerfile string
	Mounts         []Mount
	Steps          []Step
	Entrypoint     Cmd

	// From static_build. If StaticDockerfile is populated,
	// we do not expect the iterative build fields to be populated.
	StaticDockerfile string
	StaticBuildPath  string // the absolute path to the files

	Repos []LocalGithubRepo
}

func (m Manifest) ConfigMatcher() (PathMatcher, error) {
	configMatcher, err := NewSimpleFileMatcher(m.ConfigFiles...)
	if err != nil {
		return nil, err
	}
	return configMatcher, nil
}

func (m Manifest) IsStaticBuild() bool {
	return m.StaticDockerfile != ""
}

func (m Manifest) LocalPaths() []string {
	if m.IsStaticBuild() {
		return []string{m.StaticBuildPath}
	}

	result := make([]string, len(m.Mounts))
	for i, mount := range m.Mounts {
		result[i] = mount.LocalPath
	}
	return result
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

	if m.DockerRef == nil {
		return validateErrf("[validate] manifest %q missing image ref", m.Name)
	}

	if m.K8sYaml == "" {
		return validateErrf("[validate] manifest %q missing YAML file", m.Name)
	}

	if m.IsStaticBuild() {
		if m.StaticBuildPath == "" {
			return validateErrf("[validate] manifest %q missing build path", m.Name)
		}
	} else {
		if m.BaseDockerfile == "" {
			return validateErrf("[validate] manifest %q missing base dockerfile", m.Name)
		}
	}

	return nil
}

func (m1 Manifest) Equal(m2 Manifest) bool {
	primitivesMatch := m1.Name == m2.Name && m1.K8sYaml == m2.K8sYaml && m1.DockerRef == m2.DockerRef && m1.BaseDockerfile == m2.BaseDockerfile && m1.StaticDockerfile == m2.StaticDockerfile && m1.StaticBuildPath == m2.StaticBuildPath
	cmdMatch := m1.Entrypoint.Equal(m2.Entrypoint)
	pmMatch := m1.FileFilter == m2.FileFilter

	return primitivesMatch && cmdMatch && pmMatch
}

type ManifestCreator interface {
	CreateManifests(ctx context.Context, svcs []Manifest, watch bool) error
}

type Mount struct {
	LocalPath     string
	ContainerPath string
}

type LocalGithubRepo struct {
	LocalPath            string
	DockerignoreContents string
	GitignoreContents    string
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

func (c1 Cmd) Equal(c2 Cmd) bool {
	if (c1.Argv == nil) != (c2.Argv == nil) {
		return false
	}

	if len(c1.Argv) != len(c2.Argv) {
		return false
	}

	for i := range c1.Argv {
		if c1.Argv[i] != c2.Argv[i] {
			return false
		}
	}

	return true
}

func (c Cmd) Empty() bool {
	return len(c.Argv) == 0
}

func ToShellCmd(cmd string) Cmd {
	if cmd == "" {
		return Cmd{}
	}
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
