package model

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/windmilleng/tilt/internal/sliceutils"
)

type ManifestName string

func (m ManifestName) String() string { return string(m) }

// NOTE: If you modify Manifest, make sure to modify `Manifest.Equal` appropriately
type Manifest struct {
	// Properties for all manifests.
	Name          ManifestName
	tiltFilename  string
	dockerignores []Dockerignore
	repos         []LocalGithubRepo

	// Info needed to build an image. Can be StaticBuild, FastBuild... etc.
	buildInfo buildInfo

	// Info needed to deploy. Can be k8s yaml, docker compose, etc.
	deployInfo deployInfo

	// Properties for fast_build (builds that support
	// iteration based on past artifacts)
	BaseDockerfile string
	Mounts         []Mount
	Steps          []Step
	Entrypoint     Cmd

	StaticBuildPath string // the absolute path to the files
	StaticBuildArgs DockerBuildArgs
}

type DockerBuildArgs map[string]string

func (m Manifest) WithBuildInfo(info buildInfo) Manifest {
	m.buildInfo = info
	return m
}

func (m Manifest) DockerInfo() DockerInfo {
	switch info := m.buildInfo.(type) {
	case DockerInfo:
		return info
	default:
		return DockerInfo{}
	}
}

func (m Manifest) StaticBuildInfo() StaticBuild {
	dInfo := m.DockerInfo()
	if dInfo.Empty() {
		return StaticBuild{}
	}

	switch info := dInfo.buildDetails.(type) {
	case StaticBuild:
		return info
	default:
		return StaticBuild{}
	}
}

func (m Manifest) IsStaticBuild() bool {
	return !m.StaticBuildInfo().Empty()
}

func (m Manifest) FastBuildInfo() FastBuild {
	dInfo := m.DockerInfo()
	if dInfo.Empty() {
		return FastBuild{}
	}

	switch info := dInfo.buildDetails.(type) {
	case FastBuild:
		return info
	default:
		return FastBuild{}
	}
}

func (m Manifest) DCInfo() DCInfo {
	switch info := m.deployInfo.(type) {
	case DCInfo:
		return info
	default:
		return DCInfo{}
	}
}

func (m Manifest) IsDC() bool {
	return !m.DCInfo().Empty()
}

func (m Manifest) K8sInfo() K8sInfo {
	switch info := m.deployInfo.(type) {
	case K8sInfo:
		return info
	default:
		return K8sInfo{}
	}
}

func (m Manifest) IsK8s() bool {
	return !m.K8sInfo().Empty()
}

func (m Manifest) WithDeployInfo(info deployInfo) Manifest {
	m.deployInfo = info
	return m
}

func (m Manifest) WithRepos(repos []LocalGithubRepo) Manifest {
	m.repos = append(append([]LocalGithubRepo{}, m.repos...), repos...)
	return m
}

func (m Manifest) WithDockerignores(dockerignores []Dockerignore) Manifest {
	m.dockerignores = append(append([]Dockerignore{}, m.dockerignores...), dockerignores...)
	return m
}

func (m Manifest) Dockerignores() []Dockerignore {
	return append([]Dockerignore{}, m.dockerignores...)
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

// TODO: implement this (validate for container build)
func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("[validate] manifest missing name: %+v", m)
	}
	for _, m := range m.Mounts {
		if !filepath.IsAbs(m.LocalPath) {
			return fmt.Errorf(
				"[validate] mount.LocalPath must be an absolute path (got: %s)", m.LocalPath)
		}
	}
	return nil
}

// ValidateDockerK8sManifest indicates whether this manifest is a valid Docker-buildable &
// k8s-deployable manifest.
func (m Manifest) ValidateDockerK8sManifest() error {
	if m.DockerInfo().DockerRef == nil {
		return fmt.Errorf("[ValidateDockerK8sManifest] manifest %q missing image ref", m.Name)
	}

	if m.K8sInfo().YAML == "" {
		return fmt.Errorf("[ValidateDockerK8sManifest] manifest %q missing k8s YAML", m.Name)
	}

	if m.IsStaticBuild() {
		if m.StaticBuildPath == "" {
			return fmt.Errorf("[ValidateDockerK8sManifest] manifest %q missing build path", m.Name)
		}
	} else {
		if m.BaseDockerfile == "" {
			return fmt.Errorf("[ValidateDockerK8sManifest] manifest %q missing base dockerfile", m.Name)
		}
	}

	return nil
}

func (m1 Manifest) Equal(m2 Manifest) bool {
	primitivesMatch := m1.Name == m2.Name && m1.BaseDockerfile == m2.BaseDockerfile && m1.StaticBuildPath == m2.StaticBuildPath && m1.tiltFilename == m2.tiltFilename
	entrypointMatch := m1.Entrypoint.Equal(m2.Entrypoint)
	mountsMatch := reflect.DeepEqual(m1.Mounts, m2.Mounts)
	reposMatch := reflect.DeepEqual(m1.repos, m2.repos)
	stepsMatch := m1.stepsEqual(m2.Steps)
	dockerignoresMatch := reflect.DeepEqual(m1.dockerignores, m2.dockerignores)
	buildArgsMatch := reflect.DeepEqual(m1.StaticBuildArgs, m2.StaticBuildArgs)

	docker1 := m1.DockerInfo()
	docker2 := m2.DockerInfo()
	dockerEqual := reflect.DeepEqual(docker1, docker2)

	dc1 := m1.DCInfo()
	dc2 := m2.DCInfo()
	dockerComposeEqual := reflect.DeepEqual(dc1, dc2)

	k8s1 := m1.K8sInfo()
	k8s2 := m2.K8sInfo()
	k8sEqual := reflect.DeepEqual(k8s1, k8s2)

	return primitivesMatch &&
		entrypointMatch &&
		mountsMatch &&
		reposMatch &&
		stepsMatch &&
		buildArgsMatch &&
		dockerignoresMatch &&
		dockerEqual &&
		dockerComposeEqual &&
		k8sEqual
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

	deduped := sliceutils.DedupeStringSlice(deps)

	// Sort so that any nested paths come after their parents
	sort.Strings(deduped)

	return deduped
}

func (m Manifest) WithConfigFiles(confFiles []string) Manifest {
	return m
}

func (m Manifest) LocalRepos() []LocalGithubRepo {
	return m.repos
}

func (m Manifest) TiltFilename() string {
	return m.tiltFilename
}

func (m Manifest) WithTiltFilename(f string) Manifest {
	m.tiltFilename = f
	return m
}

type Mount struct {
	LocalPath     string
	ContainerPath string
}

type Dockerignore struct {
	// The path to evaluate the dockerignore contents relative to
	LocalPath string
	Contents  string
}

type LocalGithubRepo struct {
	LocalPath         string
	GitignoreContents string
}

func (LocalGithubRepo) IsRepo() {}

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

type PortForward struct {
	// The port to expose on localhost of the current machine.
	LocalPort int

	// The port to connect to inside the deployed container.
	// If 0, we will connect to the first containerPort.
	ContainerPort int
}
