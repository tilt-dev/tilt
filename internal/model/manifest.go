package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/sliceutils"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TODO(nick): We should probably get rid of ManifestName completely and just use TargetName everywhere.
type ManifestName string

func (m ManifestName) String() string         { return string(m) }
func (m ManifestName) TargetName() TargetName { return TargetName(m) }

// NOTE: If you modify Manifest, make sure to modify `Manifest.Equal` appropriately
type Manifest struct {
	// Properties for all manifests.
	Name         ManifestName
	tiltFilename string

	// Info needed to Docker build an image. (This struct contains details of StaticBuild, FastBuild... etc.)
	// (If we ever support multiple build engines, this can become an interface wildcard similar to `deployTarget`).
	ImageTarget ImageTarget

	// Info needed to deploy. Can be k8s yaml, docker compose, etc.
	deployTarget TargetSpec
}

func (m Manifest) ID() TargetID {
	return TargetID{
		Type: TargetTypeManifest,
		Name: m.Name.TargetName(),
	}
}

type DockerBuildArgs map[string]string

func (m Manifest) DockerComposeTarget() DockerComposeTarget {
	ret, _ := m.deployTarget.(DockerComposeTarget)
	return ret
}

func (m Manifest) IsDC() bool {
	_, ok := m.deployTarget.(DockerComposeTarget)
	return ok
}

func (m Manifest) K8sTarget() K8sTarget {
	ret, _ := m.deployTarget.(K8sTarget)
	return ret
}

func (m Manifest) IsK8s() bool {
	_, ok := m.deployTarget.(K8sTarget)
	return ok
}

func (m Manifest) WithDeployTarget(t TargetSpec) Manifest {
	switch typedTarget := t.(type) {
	case K8sTarget:
		typedTarget.Name = m.Name.TargetName()
		t = typedTarget
	case DockerComposeTarget:
		typedTarget.Name = m.Name.TargetName()
		t = typedTarget
	}
	m.deployTarget = t
	return m
}

func (m Manifest) LocalPaths() []string {
	// TODO(matt?) DC mounts should probably stored somewhere more consistent with Static/Fast Build
	switch di := m.deployTarget.(type) {
	case DockerComposeTarget:
		return di.LocalPaths()
	default:
		return m.ImageTarget.LocalPaths()
	}
}

func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("[validate] manifest missing name: %+v", m)
	}

	if !m.ImageTarget.ID().Empty() || m.IsK8s() {
		err := m.ImageTarget.Validate()
		if err != nil {
			return err
		}
	}

	if m.deployTarget != nil {
		err := m.deployTarget.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

func (m1 Manifest) Equal(m2 Manifest) bool {
	primitivesMatch := m1.Name == m2.Name && m1.tiltFilename == m2.tiltFilename
	dockerEqual := DeepEqual(m1.ImageTarget, m2.ImageTarget)

	dc1 := m1.DockerComposeTarget()
	dc2 := m2.DockerComposeTarget()
	dockerComposeEqual := DeepEqual(dc1, dc2)

	k8s1 := m1.K8sTarget()
	k8s2 := m2.K8sTarget()
	k8sEqual := DeepEqual(k8s1, k8s2)

	return primitivesMatch &&
		dockerEqual &&
		dockerComposeEqual &&
		k8sEqual
}

func (m Manifest) ManifestName() ManifestName {
	return m.Name
}

// TODO(nick): This method should be deleted. We should just de-dupe and sort LocalPaths once
// when we create it, rather than have a duplicate method that does the "right" thing.
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

func (m Manifest) TiltFilename() string {
	return m.tiltFilename
}

// Right now, the Tiltfile name is duplicated in the manifest and inner objects,
// but this is just a transitional state ImageTarget and DockerComposeTarget are
// their own top-level objects in the graph.
func (m Manifest) WithTiltFilename(f string) Manifest {
	m.tiltFilename = f
	return m
}

var _ TargetSpec = Manifest{}

type Mount struct {
	LocalPath     string
	ContainerPath string
}

type Dockerignore struct {
	// The path to evaluate the dockerignore contents relative to
	LocalPath string
	Contents  string
}

type LocalGitRepo struct {
	LocalPath         string
	GitignoreContents string
}

func (LocalGitRepo) IsRepo() {}

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

var imageTargetAllowUnexported = cmp.AllowUnexported(ImageTarget{})
var dcTargetAllowUnexported = cmp.AllowUnexported(DockerComposeTarget{})
var dockerRefEqual = cmp.Comparer(func(a, b reference.Named) bool {
	aNil := a == nil
	bNil := b == nil
	if aNil && bNil {
		return true
	}

	if aNil != bNil {
		return false
	}

	return a.String() == b.String()
})

func DeepEqual(x, y interface{}) bool {
	return cmp.Equal(x, y,
		cmpopts.EquateEmpty(),
		imageTargetAllowUnexported,
		dcTargetAllowUnexported,
		dockerRefEqual)
}
