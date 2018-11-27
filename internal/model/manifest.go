package model

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/yaml"
)

type ManifestName string

func (m ManifestName) String() string { return string(m) }

// NOTE: If you modify Manifest, make sure to modify `Manifest.Equal` appropriately
type Manifest struct {
	// Properties for all builds.
	Name         ManifestName
	k8sYaml      string
	tiltFilename string
	dockerRef    reference.Named
	portForwards []PortForward
	cachePaths   []string

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
	StaticBuildArgs  DockerBuildArgs

	Repos []LocalGithubRepo

	// Tiltfile Manifest specific fields
	IsTiltfile bool
}

type DockerBuildArgs map[string]string

func (m Manifest) WithCachePaths(paths []string) Manifest {
	m.cachePaths = append(append([]string{}, m.cachePaths...), paths...)
	sort.Strings(m.cachePaths)
	return m
}

func (m Manifest) CachePaths() []string {
	return append([]string{}, m.cachePaths...)
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
	if m.IsTiltfile {
		return nil
	}
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

	if m.dockerRef == nil {
		return validateErrf("[validate] manifest %q missing image ref", m.Name)
	}

	if m.K8sYAML() == "" {
		return validateErrf("[validate] manifest %q missing k8s YAML", m.Name)
	}

	for _, m := range m.Mounts {
		if !filepath.IsAbs(m.LocalPath) {
			return validateErrf(
				"[validate] mount.LocalPath must be an absolute path (got: %s)", m.LocalPath)
		}
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
	primitivesMatch := m1.Name == m2.Name && m1.k8sYaml == m2.k8sYaml && m1.dockerRef == m2.dockerRef && m1.BaseDockerfile == m2.BaseDockerfile && m1.StaticDockerfile == m2.StaticDockerfile && m1.StaticBuildPath == m2.StaticBuildPath && m1.tiltFilename == m2.tiltFilename && m1.IsTiltfile == m2.IsTiltfile
	entrypointMatch := m1.Entrypoint.Equal(m2.Entrypoint)
	mountsMatch := m1.mountsEqual(m2.Mounts)
	reposMatch := m1.reposEqual(m2.Repos)
	stepsMatch := m1.stepsEqual(m2.Steps)
	portForwardsMatch := m1.portForwardsEqual(m2)
	buildArgsMatch := reflect.DeepEqual(m1.StaticBuildArgs, m2.StaticBuildArgs)
	cachePathsMatch := stringSlicesEqual(m1.cachePaths, m2.cachePaths)

	return primitivesMatch && entrypointMatch && mountsMatch && reposMatch && portForwardsMatch && stepsMatch && buildArgsMatch && cachePathsMatch
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range b {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func (m1 Manifest) mountsEqual(m2 []Mount) bool {
	if (m1.Mounts == nil) != (m2 == nil) {
		return false
	}

	if len(m1.Mounts) != len(m2) {
		return false
	}

	for i := range m2 {
		if m1.Mounts[i] != m2[i] {
			return false
		}
	}

	return true
}

func (m1 Manifest) reposEqual(m2 []LocalGithubRepo) bool {
	if (m1.Repos == nil) != (m2 == nil) {
		return false
	}

	if len(m1.Repos) != len(m2) {
		return false
	}

	for i := range m2 {
		if m1.Repos[i] != m2[i] {
			return false
		}
	}

	return true
}

func (m1 Manifest) portForwardsEqual(m2 Manifest) bool {
	if len(m1.portForwards) != len(m2.portForwards) {
		return false
	}

	for i := range m2.portForwards {
		if m1.portForwards[i] != m2.portForwards[i] {
			return false
		}
	}

	return true
}

func (m1 Manifest) stepsEqual(s2 []Step) bool {
	if len(m1.Steps) != len(s2) {
		return false
	}

	for i := range s2 {
		if !m1.Steps[i].Equal(s2[i]) {
			return false
		}
	}

	return true
}

func (m Manifest) ManifestName() ManifestName {
	return m.Name
}

func (m Manifest) Dependencies() []string {
	// TODO(dmiller) we can know the length of this slice
	deps := []string{}

	for _, p := range m.LocalPaths() {
		deps = append(deps, p)
	}

	return sliceutils.DedupeStringSlice(deps)
}

func (m Manifest) WithConfigFiles(confFiles []string) Manifest {
	return m
}

func (m Manifest) LocalRepos() []LocalGithubRepo {
	return m.Repos
}

func (m Manifest) WithPortForwards(pf []PortForward) Manifest {
	m.portForwards = pf
	return m
}

func (m Manifest) PortForwards() []PortForward {
	return m.portForwards
}

func (m Manifest) TiltFilename() string {
	return m.tiltFilename
}

func (m Manifest) WithTiltFilename(f string) Manifest {
	m.tiltFilename = f
	return m
}

func (m Manifest) K8sYAML() string {
	return m.k8sYaml
}

func (m Manifest) AppendK8sYAML(y string) Manifest {
	if m.k8sYaml == "" {
		return m.WithK8sYAML(y)
	}
	if y == "" {
		return m
	}

	return m.WithK8sYAML(yaml.ConcatYAML(m.k8sYaml, y))
}

func (m Manifest) WithK8sYAML(y string) Manifest {
	m.k8sYaml = y
	return m
}

func (m Manifest) DockerRef() reference.Named {
	return m.dockerRef
}

func (m Manifest) WithDockerRef(ref reference.Named) Manifest {
	m.dockerRef = ref
	return m
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

func (r1 LocalGithubRepo) Equal(r2 LocalGithubRepo) bool {
	return r1.DockerignoreContents == r2.DockerignoreContents && r1.GitignoreContents == r2.GitignoreContents && r1.LocalPath == r2.LocalPath
}

type Step struct {
	// Required. The command to run in this step.
	Cmd Cmd
	// Optional. If not specified, this step runs on every change.
	// If specified, we only run the Cmd if the trigger matches the changed file.
	Triggers []string
	// Directory the Triggers are relative to
	BaseDirectory string
}

func (s1 Step) Equal(s2 Step) bool {
	if s1.BaseDirectory != s2.BaseDirectory {
		return false
	}

	if !s1.Cmd.Equal(s2.Cmd) {
		return false
	}

	if len(s1.Triggers) != len(s2.Triggers) {
		return false
	}

	for i := range s2.Triggers {
		if s1.Triggers[i] != s2.Triggers[i] {
			return false
		}
	}

	return true
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

func ToStep(cwd string, cmd Cmd) Step {
	return Step{BaseDirectory: cwd, Cmd: cmd}
}

func ToSteps(cwd string, cmds []Cmd) []Step {
	res := make([]Step, len(cmds))
	for i, cmd := range cmds {
		res[i] = ToStep(cwd, cmd)
	}
	return res
}

func ToShellSteps(cwd string, cmds []string) []Step {
	return ToSteps(cwd, ToShellCmds(cmds))
}

type ValidateErr struct {
	s string
}

func (e *ValidateErr) Error() string { return e.s }

var _ error = &ValidateErr{}

func validateErrf(format string, a ...interface{}) *ValidateErr {
	return &ValidateErr{s: fmt.Sprintf(format, a...)}
}

type PortForward struct {
	// The port to expose on localhost of the current machine.
	LocalPort int

	// The port to connect to inside the deployed container.
	// If 0, we will connect to the first containerPort.
	ContainerPort int
}
