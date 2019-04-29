package model

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/docker/distribution/reference"

	"github.com/windmilleng/tilt/internal/container"
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
	Name ManifestName

	// Info needed to build an image. (This struct contains details of DockerBuild, FastBuild... etc.)
	ImageTargets []ImageTarget

	// Info needed to deploy. Can be k8s yaml, docker compose, etc.
	deployTarget TargetSpec

	UpdateMode UpdateMode
}

func (m Manifest) ID() TargetID {
	return TargetID{
		Type: TargetTypeManifest,
		Name: m.Name.TargetName(),
	}
}

func (m Manifest) DependencyIDs() []TargetID {
	result := []TargetID{}
	for _, iTarget := range m.ImageTargets {
		result = append(result, iTarget.ID())
	}
	if !m.deployTarget.ID().Empty() {
		result = append(result, m.deployTarget.ID())
	}
	return result
}

func (m Manifest) WithImageTarget(iTarget ImageTarget) Manifest {
	m.ImageTargets = []ImageTarget{iTarget}
	return m
}

func (m Manifest) WithImageTargets(iTargets []ImageTarget) Manifest {
	m.ImageTargets = append([]ImageTarget{}, iTargets...)
	return m
}

func (m Manifest) ImageTargetAt(i int) ImageTarget {
	if i < len(m.ImageTargets) {
		return m.ImageTargets[i]
	}
	return ImageTarget{}
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

func (m Manifest) DeployTarget() TargetSpec {
	return m.deployTarget
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

func (m Manifest) TargetSpecs() []TargetSpec {
	result := []TargetSpec{}
	for _, t := range m.ImageTargets {
		result = append(result, t)
	}
	result = append(result, m.deployTarget)
	return result
}

func (m Manifest) IsImageDeployed(iTarget ImageTarget) bool {
	id := iTarget.ID()
	for _, depID := range m.DeployTarget().DependencyIDs() {
		if depID == id {
			return true
		}
	}
	return false
}

func (m Manifest) LocalPaths() []string {
	// TODO(matt?) DC syncs should probably stored somewhere more consistent with Docker/Fast Build
	switch di := m.deployTarget.(type) {
	case DockerComposeTarget:
		return di.LocalPaths()
	default:
		paths := []string{}
		for _, iTarget := range m.ImageTargets {
			paths = append(paths, iTarget.LocalPaths()...)
		}
		return sliceutils.DedupedAndSorted(paths)
	}
}

func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("[validate] manifest missing name: %+v", m)
	}

	for _, iTarget := range m.ImageTargets {
		err := iTarget.Validate()
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
	primitivesMatch := m1.Name == m2.Name
	dockerEqual := DeepEqual(m1.ImageTargets, m2.ImageTargets)

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

func (m Manifest) Empty() bool {
	return m.Equal(Manifest{})
}

var _ TargetSpec = Manifest{}

type Sync struct {
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

type Run struct {
	// Required. The command to run.
	Cmd Cmd
	// Optional. If not specified, this command runs on every change.
	// If specified, we only run the Cmd if the changed file matches a trigger.
	Triggers PathSet
}

func (r Run) WithTriggers(paths []string, baseDir string) Run {
	if len(paths) > 0 {
		r.Triggers = PathSet{
			Paths:         paths,
			BaseDirectory: baseDir,
		}
	} else {
		r.Triggers = PathSet{}
	}
	return r
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

func ToRun(cmd Cmd) Run {
	return Run{Cmd: cmd}
}

func ToRuns(cmds []Cmd) []Run {
	res := make([]Run, len(cmds))
	for i, cmd := range cmds {
		res[i] = ToRun(cmd)
	}
	return res
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
var labelRequirementAllowUnexported = cmp.AllowUnexported(labels.Requirement{})
var k8sTargetAllowUnexported = cmp.AllowUnexported(K8sTarget{})
var selectorAllowUnexported = cmp.AllowUnexported(container.RefSelector{})

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
		labelRequirementAllowUnexported,
		k8sTargetAllowUnexported,
		selectorAllowUnexported,
		dockerRefEqual)
}

type UpdateMode int

const (
	UpdateModeAuto   UpdateMode = iota
	UpdateModeManual UpdateMode = iota
)
