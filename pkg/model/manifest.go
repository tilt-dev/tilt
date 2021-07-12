package model

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/docker/distribution/reference"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/sliceutils"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TODO(nick): We should probably get rid of ManifestName completely and just use TargetName everywhere.
type ManifestName string
type ManifestNameSet map[ManifestName]bool

func ManifestNames(names []string) []ManifestName {
	mNames := make([]ManifestName, len(names))
	for i, name := range names {
		mNames[i] = ManifestName(name)
	}
	return mNames
}

const TiltfileManifestName = ManifestName("(Tiltfile)")

func (m ManifestName) String() string         { return string(m) }
func (m ManifestName) TargetName() TargetName { return TargetName(m) }

// NOTE: If you modify Manifest, make sure to modify `Manifest.Equal` appropriately
type Manifest struct {
	// Properties for all manifests.
	Name ManifestName

	// Info needed to build an image. (This struct contains details of DockerBuild, CustomBuild... etc.)
	ImageTargets []ImageTarget

	// Info needed to deploy. Can be k8s yaml, docker compose, etc.
	DeployTarget TargetSpec

	// How updates are triggered:
	// - automatically, when we detect a change
	// - manually, only when the user tells us to
	TriggerMode TriggerMode

	// The resource in this manifest will not be built until all of its dependencies have been
	// ready at least once.
	ResourceDependencies []ManifestName

	Source ManifestSource

	Labels []string
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
	if !m.DeployTarget.ID().Empty() {
		result = append(result, m.DeployTarget.ID())
	}
	return result
}

// A map from each target id to the target IDs that depend on it.
func (m Manifest) ReverseDependencyIDs() map[TargetID][]TargetID {
	result := make(map[TargetID][]TargetID)
	for _, iTarget := range m.ImageTargets {
		for _, depID := range iTarget.DependencyIDs() {
			result[depID] = append(result[depID], iTarget.ID())
		}
	}
	if !m.DeployTarget.ID().Empty() {
		for _, depID := range m.DeployTarget.DependencyIDs() {
			result[depID] = append(result[depID], m.DeployTarget.ID())
		}
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

func (m Manifest) ImageTargetWithID(id TargetID) ImageTarget {
	for _, target := range m.ImageTargets {
		if target.ID() == id {
			return target
		}
	}
	return ImageTarget{}
}

type DockerBuildArgs map[string]string

func (m Manifest) LocalTarget() LocalTarget {
	ret, _ := m.DeployTarget.(LocalTarget)
	return ret
}

func (m Manifest) IsLocal() bool {
	_, ok := m.DeployTarget.(LocalTarget)
	return ok
}

func (m Manifest) DockerComposeTarget() DockerComposeTarget {
	ret, _ := m.DeployTarget.(DockerComposeTarget)
	return ret
}

func (m Manifest) IsDC() bool {
	_, ok := m.DeployTarget.(DockerComposeTarget)
	return ok
}

func (m Manifest) K8sTarget() K8sTarget {
	ret, _ := m.DeployTarget.(K8sTarget)
	return ret
}

func (m Manifest) IsK8s() bool {
	_, ok := m.DeployTarget.(K8sTarget)
	return ok
}

func (m Manifest) PodReadinessMode() PodReadinessMode {
	if k8sTarget, ok := m.DeployTarget.(K8sTarget); ok {
		return k8sTarget.PodReadinessMode
	}
	return PodReadinessNone
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
	m.DeployTarget = t
	return m
}

func (m Manifest) WithTriggerMode(mode TriggerMode) Manifest {
	m.TriggerMode = mode
	return m
}

func (m Manifest) TargetIDSet() map[TargetID]bool {
	result := make(map[TargetID]bool)
	specs := m.TargetSpecs()
	for _, spec := range specs {
		result[spec.ID()] = true
	}
	return result
}

func (m Manifest) TargetSpecs() []TargetSpec {
	result := []TargetSpec{}
	for _, t := range m.ImageTargets {
		result = append(result, t)
	}
	if m.DeployTarget != nil {
		result = append(result, m.DeployTarget)
	}
	return result
}

func (m Manifest) IsImageDeployed(iTarget ImageTarget) bool {
	id := iTarget.ID()
	for _, depID := range m.DeployTarget.DependencyIDs() {
		if depID == id {
			return true
		}
	}
	return false
}

func (m Manifest) LocalPaths() []string {
	// TODO(matt?) DC syncs should probably stored somewhere more consistent with Docker/Custom
	switch di := m.DeployTarget.(type) {
	case DockerComposeTarget:
		return di.LocalPaths()
	case LocalTarget:
		return di.Dependencies()
	case ImageTarget, K8sTarget:
		// fall through to paths for image targets, below
	}
	paths := []string{}
	for _, iTarget := range m.ImageTargets {
		paths = append(paths, iTarget.LocalPaths()...)
	}
	return sliceutils.DedupedAndSorted(paths)
}

func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("[validate] manifest missing name: %+v", m)
	}

	if errs := path.ValidatePathSegmentName(m.Name.String(), false); len(errs) != 0 {
		return fmt.Errorf("invalid value %q: %v", m.Name.String(), errs[0])
	}

	for _, iTarget := range m.ImageTargets {
		err := iTarget.Validate()
		if err != nil {
			return err
		}
	}

	if m.DeployTarget != nil {
		err := m.DeployTarget.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// ChangesInvalidateBuild checks whether the changes from old => new manifest
// invalidate our build of the old one; i.e. if we're replacing `old` with `new`,
// should we perform a full rebuild?
func ChangesInvalidateBuild(old, new Manifest) bool {
	dockerEq, k8sEq, dcEq, localEq := old.fieldGroupsEqualForBuildInvalidation(new)

	return !dockerEq || !k8sEq || !dcEq || !localEq
}

// Compare all fields that might invalidate a build
func (m1 Manifest) fieldGroupsEqualForBuildInvalidation(m2 Manifest) (dockerEq, k8sEq, dcEq, localEq bool) {
	dockerEq = equalForBuildInvalidation(m1.ImageTargets, m2.ImageTargets)

	dc1 := m1.DockerComposeTarget()
	dc2 := m2.DockerComposeTarget()
	dcEq = equalForBuildInvalidation(dc1, dc2)

	k8s1 := m1.K8sTarget()
	k8s2 := m2.K8sTarget()
	k8sEq = equalForBuildInvalidation(k8s1, k8s2)

	lt1 := m1.LocalTarget()
	lt2 := m2.LocalTarget()
	localEq = equalForBuildInvalidation(lt1, lt2)

	return dockerEq, dcEq, k8sEq, localEq
}

func (m Manifest) ManifestName() ManifestName {
	return m.Name
}

func LocalRefSelectorsForManifests(manifests []Manifest) []container.RefSelector {
	var res []container.RefSelector
	for _, m := range manifests {
		for _, iTarg := range m.ImageTargets {
			sel := container.NameSelector(iTarg.Refs.LocalRef()).WithNameMatch()
			res = append(res, sel)
		}
	}
	return res
}

var _ TargetSpec = Manifest{}

type Sync struct {
	LocalPath     string
	ContainerPath string
}

type LocalGitRepo struct {
	LocalPath string
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

type PortForward struct {
	// The port to connect to inside the deployed container.
	// If 0, we will connect to the first containerPort.
	ContainerPort int

	// The port to expose on the current machine.
	LocalPort int

	// Optional host to bind to on the current machine (localhost by default)
	Host string

	// Optional name of the port forward; if given, used as text of the URL
	// displayed in the web UI (e.g. <a href="localhost:8888">Debugger</a>)
	Name string

	// Optional path at the port forward that we link to in UIs
	// (useful if e.g. nothing lives at "/" and devs will always
	// want "localhost:xxxx/v1/app")
	// (Private with getter/setter b/c may be nil.)
	path *url.URL
}

func (pf PortForward) PathForAppend() string {
	if pf.path == nil {
		return ""
	}
	return strings.TrimPrefix(pf.path.String(), "/")
}

func (pf PortForward) WithPath(p *url.URL) PortForward {
	pf.path = p
	return pf
}

func MustPortForward(local int, container int, host string, name string, path string) PortForward {
	var parsedPath *url.URL
	var err error
	if path != "" {
		parsedPath, err = url.Parse(path)
		if err != nil {
			panic(err)
		}
	}
	return PortForward{
		ContainerPort: container,
		LocalPort:     local,
		Host:          host,
		Name:          name,
		path:          parsedPath,
	}
}

// A link associated with resource; may represent a port forward, an endpoint
// derived from a Service/Ingress/etc., or a URL manually associated with a
// resource via the Tiltfile
type Link struct {
	URL *url.URL

	// Optional name of the link; if given, used as text of the URL
	// displayed in the web UI (e.g. <a href="localhost:8888">Debugger</a>)
	Name string
}

func (li Link) URLString() string { return li.URL.String() }

func NewLink(urlStr string, name string) (Link, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return Link{}, errors.Wrapf(err, "parsing URL %q", urlStr)
	}
	return Link{
		URL:  u,
		Name: name,
	}, nil
}

func MustNewLink(urlStr string, name string) Link {
	li, err := NewLink(urlStr, name)
	if err != nil {
		panic(err)
	}
	return li
}

// ByURL implements sort.Interface based on the URL field.
type ByURL []Link

func (lns ByURL) Len() int           { return len(lns) }
func (lns ByURL) Less(i, j int) bool { return lns[i].URLString() < lns[j].URLString() }
func (lns ByURL) Swap(i, j int)      { lns[i], lns[j] = lns[j], lns[i] }

func (pf PortForward) ToLink() Link {
	host := pf.Host
	if host == "" {
		host = "localhost"
	}
	url := fmt.Sprintf("http://%s:%d/%s", host, pf.LocalPort, pf.PathForAppend())

	// We panic on error here because we provide the URL format ourselves,
	// so if it's bad, something is very wrong.
	return MustNewLink(url, pf.Name)
}

func LinksToURLStrings(lns []Link) []string {
	res := make([]string, len(lns))
	for i, ln := range lns {
		res[i] = ln.URLString()
	}
	return res
}

var imageTargetAllowUnexported = cmp.AllowUnexported(ImageTarget{})
var dcTargetAllowUnexported = cmp.AllowUnexported(DockerComposeTarget{})
var labelRequirementAllowUnexported = cmp.AllowUnexported(labels.Requirement{})
var k8sTargetAllowUnexported = cmp.AllowUnexported(K8sTarget{})
var localTargetAllowUnexported = cmp.AllowUnexported(LocalTarget{})
var selectorAllowUnexported = cmp.AllowUnexported(container.RefSelector{})
var refSetAllowUnexported = cmp.AllowUnexported(container.RefSet{})
var registryAllowUnexported = cmp.AllowUnexported(container.Registry{})
var portForwardPathAllowUnexported = cmp.AllowUnexported(PortForward{})
var ignoreCustomBuildDepsField = cmpopts.IgnoreFields(CustomBuild{}, "Deps")
var ignoreLocalTargetDepsField = cmpopts.IgnoreFields(LocalTarget{}, "Deps")
var ignoreDockerBuildCacheFrom = cmpopts.IgnoreFields(DockerBuild{}, "CacheFrom")

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

// Determine whether interfaces x and y are equal, excluding fields that don't invalidate a build.
func equalForBuildInvalidation(x, y interface{}) bool {
	return cmp.Equal(x, y,
		cmpopts.EquateEmpty(),
		imageTargetAllowUnexported,
		dcTargetAllowUnexported,
		labelRequirementAllowUnexported,
		k8sTargetAllowUnexported,
		localTargetAllowUnexported,
		selectorAllowUnexported,
		refSetAllowUnexported,
		registryAllowUnexported,
		portForwardPathAllowUnexported,
		dockerRefEqual,

		// deps changes don't invalidate a build, so don't compare fields used only for deps
		ignoreCustomBuildDepsField,
		ignoreLocalTargetDepsField,

		// DockerBuild.CacheFrom doesn't invalidate a build (b/c it affects HOW we build but
		// shouldn't affect the result of the build), so don't compare these fields
		ignoreDockerBuildCacheFrom,
	)
}

type ManifestSource string

const ManifestSourceTiltfile = ManifestSource("")
const ManifestSourceMetrics = ManifestSource("metrics")
