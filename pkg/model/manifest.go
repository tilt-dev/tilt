package model

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/distribution/reference"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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

const MainTiltfileManifestName = ManifestName("(Tiltfile)")

func (m ManifestName) String() string         { return string(m) }
func (m ManifestName) TargetName() TargetName { return TargetName(m) }
func (m ManifestName) TargetID() TargetID {
	return TargetID{
		Type: TargetTypeManifest,
		Name: m.TargetName(),
	}
}

// NOTE: If you modify Manifest, make sure to modify `equalForBuildInvalidation` appropriately
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

	SourceTiltfile ManifestName

	Labels map[string]string
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
	switch di := m.DeployTarget.(type) {
	case LocalTarget:
		return di.Dependencies()
	case ImageTarget, K8sTarget, DockerComposeTarget:
		// fall through to paths for image targets, below
	}
	paths := []string{}
	for _, iTarget := range m.ImageTargets {
		paths = append(paths, iTarget.LocalPaths()...)
	}
	return sliceutils.DedupedAndSorted(paths)
}

func (m Manifest) WithLabels(labels map[string]string) Manifest {
	m.Labels = make(map[string]string)
	for k, v := range labels {
		m.Labels[k] = v
	}
	return m
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

func (m *Manifest) ClusterName() string {
	if m.IsDC() {
		return v1alpha1.ClusterNameDocker
	}
	if m.IsK8s() {
		return v1alpha1.ClusterNameDefault
	}
	return ""
}

// Infer image properties for each image.
func (m *Manifest) inferImageProperties(clusterImageNeeds func(TargetID) v1alpha1.ClusterImageNeeds) error {
	var deployImageIDs []TargetID
	if m.DeployTarget != nil {
		deployImageIDs = m.DeployTarget.DependencyIDs()
	}
	deployImageIDSet := make(map[TargetID]bool, len(deployImageIDs))
	for _, depID := range deployImageIDs {
		deployImageIDSet[depID] = true
	}

	for i, iTarget := range m.ImageTargets {
		iTarget, err := iTarget.inferImageProperties(
			clusterImageNeeds(iTarget.ID()), m.ClusterName())
		if err != nil {
			return fmt.Errorf("manifest %s: %v", m.Name, err)
		}

		m.ImageTargets[i] = iTarget
	}
	return nil
}

// Assemble selectors that point to other API objects created by this manifest.
func (m *Manifest) InferLiveUpdateSelectors() error {
	dag, err := NewTargetGraph(m.TargetSpecs())
	if err != nil {
		return err
	}

	for i, iTarget := range m.ImageTargets {
		luSpec := iTarget.LiveUpdateSpec
		luName := iTarget.LiveUpdateName
		if luName == "" || (len(luSpec.Syncs) == 0 && len(luSpec.Execs) == 0) {
			continue
		}

		if m.IsK8s() {
			kSelector := luSpec.Selector.Kubernetes
			if kSelector == nil {
				kSelector = &v1alpha1.LiveUpdateKubernetesSelector{}
				luSpec.Selector.Kubernetes = kSelector
			}

			if kSelector.ApplyName == "" {
				kSelector.ApplyName = m.Name.String()
			}
			if kSelector.DiscoveryName == "" {
				kSelector.DiscoveryName = m.Name.String()
			}

			// infer a selector from the ImageTarget if a container name
			// selector was not specified (currently, this is always the case
			// except in some k8s_custom_deploy configurations)
			if kSelector.ContainerName == "" {
				if iTarget.IsLiveUpdateOnly {
					// use the selector (image name) as-is; Tilt isn't building
					// this image, so no image name rewriting will occur
					kSelector.Image = iTarget.Selector
				} else {
					// refer to the ImageMap so that the LU reconciler can find
					// the true image name after any registry rewriting
					kSelector.ImageMapName = iTarget.ImageMapName()
				}
			}
		}

		if m.IsDC() {
			dcSelector := luSpec.Selector.DockerCompose
			if dcSelector == nil {
				dcSelector = &v1alpha1.LiveUpdateDockerComposeSelector{}
				luSpec.Selector.DockerCompose = dcSelector
			}

			if dcSelector.Service == "" {
				dcSelector.Service = m.Name.String()
			}
		}

		luSpec.Sources = nil
		err := dag.VisitTree(iTarget, func(dep TargetSpec) error {
			// Relies on the idea that ImageTargets creates
			// FileWatches and ImageMaps related to the ImageTarget ID.
			id := dep.ID()
			fw := id.String()

			// LiveUpdateOnly targets do NOT have an associated image map
			var imageMap string
			if depImg, ok := dep.(ImageTarget); ok && !depImg.IsLiveUpdateOnly {
				imageMap = id.Name.String()
			}

			luSpec.Sources = append(luSpec.Sources, v1alpha1.LiveUpdateSource{
				FileWatch: fw,
				ImageMap:  imageMap,
			})
			return nil
		})
		if err != nil {
			return err
		}

		iTarget.LiveUpdateSpec = luSpec
		m.ImageTargets[i] = iTarget
	}
	return nil
}

// Set DisableSource for any pieces of the manifest that are disable-able but not yet in the API
func (m Manifest) WithDisableSource(disableSource *v1alpha1.DisableSource) Manifest {
	if lt, ok := m.DeployTarget.(LocalTarget); ok {
		lt.ServeCmdDisableSource = disableSource
		m.DeployTarget = lt
	}
	return m
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

func LocalRefSelectorsForManifests(manifests []Manifest, clusters map[string]*v1alpha1.Cluster) []container.RefSelector {
	var res []container.RefSelector
	for _, m := range manifests {
		cluster := clusters[m.ClusterName()]
		for _, iTarg := range m.ImageTargets {
			refs, err := iTarg.Refs(cluster)
			if err != nil {
				// silently ignore any invalid image references because this
				// logic is only used for Docker pruning, and we can't prune
				// something invalid anyway
				continue
			}
			sel := container.NameSelector(refs.LocalRef())
			res = append(res, sel)
		}
	}
	return res
}

var _ TargetSpec = Manifest{}

// Self-contained spec for syncing files from local to a container.
//
// Unlike v1alpha1.LiveUpdateSync, all fields of this object must be absolute
// paths.
type Sync struct {
	LocalPath     string
	ContainerPath string
}

// Self-contained spec for running in a container.
//
// Unlike v1alpha1.LiveUpdateExec, all fields of this object must be absolute
// paths.
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

func PortForwardToLink(pf v1alpha1.Forward) Link {
	host := pf.Host
	if host == "" {
		host = "localhost"
	}
	u := fmt.Sprintf("http://%s:%d/%s", host, pf.LocalPort, strings.TrimPrefix(pf.Path, "/"))

	// We panic on error here because we provide the URL format ourselves,
	// so if it's bad, something is very wrong.
	return MustNewLink(u, pf.Name)
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
var portForwardPathAllowUnexported = cmp.AllowUnexported(PortForward{})
var ignoreCustomBuildDepsField = cmpopts.IgnoreFields(CustomBuild{}, "Deps")
var ignoreLocalTargetDepsField = cmpopts.IgnoreFields(LocalTarget{}, "Deps")
var ignoreDockerBuildCacheFrom = cmpopts.IgnoreFields(DockerBuild{}, "CacheFrom")
var ignoreLabels = cmpopts.IgnoreFields(Manifest{}, "Labels")
var ignoreDockerComposeProject = cmpopts.IgnoreFields(v1alpha1.DockerComposeServiceSpec{}, "Project")
var ignoreRegistryFields = cmpopts.IgnoreFields(v1alpha1.RegistryHosting{}, "HostFromClusterNetwork", "Help")

// ignoreLinks ignores user-defined links for the purpose of build invalidation
//
// This is done both because they don't actually invalidate the build AND because url.URL is not directly comparable
// in all cases (e.g. a URL with a user@ value will result in url.URL->User being populated which has unexported fields).
var ignoreLinks = cmpopts.IgnoreTypes(Link{})

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
		portForwardPathAllowUnexported,
		dockerRefEqual,

		// deps changes don't invalidate a build, so don't compare fields used only for deps
		ignoreCustomBuildDepsField,
		ignoreLocalTargetDepsField,

		// DockerBuild.CacheFrom doesn't invalidate a build (b/c it affects HOW we build but
		// shouldn't affect the result of the build), so don't compare these fields
		ignoreDockerBuildCacheFrom,

		// user-added labels don't invalidate a build
		ignoreLabels,

		// user-added links don't invalidate a build
		ignoreLinks,

		// We don't want a change to the DockerCompose Project to invalidate
		// all individual services. We track the service-specific YAML with
		// a separate ServiceYAML field.
		ignoreDockerComposeProject,

		// the RegistryHosting spec includes informational fields (Help) as
		// well as some unused by Tilt (HostFromClusterNetwork)
		ignoreRegistryFields,
	)
}

// Infer image properties for each image in the manifest set.
func InferImageProperties(manifests []Manifest) error {
	deployImageIDSet := make(map[TargetID]bool, len(manifests))
	for _, m := range manifests {
		if m.DeployTarget != nil {
			for _, depID := range m.DeployTarget.DependencyIDs() {
				deployImageIDSet[depID] = true
			}
		}
	}

	// An image only needs to be pushed if it's used in-cluster.
	// If it needs to be pushed for one manifest, it needs to be pushed for all.
	// The caching system will make sure it's not pushed multiple times.
	clusterImageNeeds := func(id TargetID) v1alpha1.ClusterImageNeeds {
		if deployImageIDSet[id] {
			return v1alpha1.ClusterImageNeedsPush
		}
		return v1alpha1.ClusterImageNeedsBase
	}

	for _, m := range manifests {
		if err := m.inferImageProperties(clusterImageNeeds); err != nil {
			return err
		}
	}
	return nil
}
