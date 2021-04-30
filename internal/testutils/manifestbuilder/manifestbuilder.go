package manifestbuilder

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Builds Manifest objects for testing.
//
// To create well-formed manifests, we want to make sure that:
// - The relationships between targets are internally consistent
//   (e.g., if there's an ImageTarget and a K8sTarget in the manifest, then
//    the K8sTarget should depend on the ImageTarget).
// - Any filepaths in the manifest are scoped to the
//   test directory (e.g., we're not trying to watch random directories
//   outside the test environment).

type ManifestBuilder struct {
	f    Fixture
	name model.ManifestName

	k8sPodReadiness    model.PodReadinessMode
	k8sYAML            string
	k8sPodSelectors    []labels.Set
	k8sImageLocators   []k8s.ImageLocator
	dcConfigPaths      []string
	localCmd           string
	localServeCmd      string
	localDeps          []string
	localAllowParallel bool
	resourceDeps       []string
	triggerMode        model.TriggerMode

	iTargets []model.ImageTarget
}

func New(f Fixture, name model.ManifestName) ManifestBuilder {
	k8sPodReadiness := model.PodReadinessWait

	// TODO(nick): A better solution would be to identify whether this
	// is a workload based on the YAML.
	if name == model.UnresourcedYAMLManifestName {
		k8sPodReadiness = model.PodReadinessIgnore
	}

	return ManifestBuilder{
		f:               f,
		name:            name,
		k8sPodReadiness: k8sPodReadiness,
	}
}

func (b ManifestBuilder) WithNamedJSONPathImageLocator(name, path string) ManifestBuilder {
	selector := k8s.MustNameSelector(name)
	jp := k8s.MustJSONPathImageLocator(selector, path)
	b.k8sImageLocators = append(b.k8sImageLocators, jp)
	return b
}

func (b ManifestBuilder) WithK8sPodReadiness(pr model.PodReadinessMode) ManifestBuilder {
	b.k8sPodReadiness = pr
	return b
}

func (b ManifestBuilder) WithK8sYAML(yaml string) ManifestBuilder {
	b.k8sYAML = yaml
	return b
}

func (b ManifestBuilder) WithK8sPodSelectors(podSelectors []labels.Set) ManifestBuilder {
	b.k8sPodSelectors = podSelectors
	return b
}

func (b ManifestBuilder) WithDockerCompose() ManifestBuilder {
	b.dcConfigPaths = []string{b.f.JoinPath("docker-compose.yml")}
	return b
}

func (b ManifestBuilder) WithLocalResource(cmd string, deps []string) ManifestBuilder {
	b.localCmd = cmd
	b.localDeps = deps
	return b
}

func (b ManifestBuilder) WithLocalServeCmd(cmd string) ManifestBuilder {
	b.localServeCmd = cmd
	return b
}

func (b ManifestBuilder) WithLocalAllowParallel(v bool) ManifestBuilder {
	b.localAllowParallel = v
	return b
}

func (b ManifestBuilder) WithTriggerMode(tm model.TriggerMode) ManifestBuilder {
	b.triggerMode = tm
	return b
}

func (b ManifestBuilder) WithImageTarget(iTarg model.ImageTarget) ManifestBuilder {
	b.iTargets = append(b.iTargets, iTarg)
	return b
}

func (b ManifestBuilder) WithImageTargets(iTargs ...model.ImageTarget) ManifestBuilder {
	b.iTargets = append(b.iTargets, iTargs...)
	return b
}

func (b ManifestBuilder) WithLiveUpdate(lu model.LiveUpdate) ManifestBuilder {
	return b.WithLiveUpdateAtIndex(lu, 0)
}

func (b ManifestBuilder) WithLiveUpdateAtIndex(lu model.LiveUpdate, index int) ManifestBuilder {
	if len(b.iTargets) <= index {
		b.f.T().Fatalf("WithLiveUpdateAtIndex: index %d out of range -- (manifestBuilder has %d image targets)", index, len(b.iTargets))
	}

	iTarg := b.iTargets[index]
	switch bd := iTarg.BuildDetails.(type) {
	case model.DockerBuild:
		bd.LiveUpdate = lu
		b.iTargets[index] = iTarg.WithBuildDetails(bd)
	case model.CustomBuild:
		bd.LiveUpdate = lu
		b.iTargets[index] = iTarg.WithBuildDetails(bd)
	default:
		b.f.T().Fatalf("unrecognized buildDetails type: %v", bd)
	}
	return b
}

func (b ManifestBuilder) WithResourceDeps(deps ...string) ManifestBuilder {
	b.resourceDeps = deps
	return b
}

func (b ManifestBuilder) Build() model.Manifest {
	var m model.Manifest

	var rds []model.ManifestName
	for _, dep := range b.resourceDeps {
		rds = append(rds, model.ManifestName(dep))
	}

	if b.k8sYAML != "" {
		k8sTarget := k8s.MustTarget(model.TargetName(b.name), b.k8sYAML)
		k8sTarget.ExtraPodSelectors = b.k8sPodSelectors
		for _, locator := range b.k8sImageLocators {
			k8sTarget.ImageLocators = append(k8sTarget.ImageLocators, locator)
		}
		k8sTarget.PodReadinessMode = b.k8sPodReadiness

		m = assembleK8s(
			model.Manifest{Name: b.name, ResourceDependencies: rds},
			k8sTarget,
			b.iTargets...)
	} else if len(b.dcConfigPaths) > 0 {
		m = assembleDC(
			model.Manifest{Name: b.name, ResourceDependencies: rds},
			model.DockerComposeTarget{
				Name:        model.TargetName(b.name),
				ConfigPaths: b.dcConfigPaths,
			},
			b.iTargets...)
	} else if b.localCmd != "" || b.localServeCmd != "" {
		updateCmd := model.ToHostCmd(b.localCmd)
		updateCmd.Dir = b.f.Path()

		serveCmd := model.ToHostCmd(b.localServeCmd)
		serveCmd.Dir = b.f.Path()

		lt := model.NewLocalTarget(
			model.TargetName(b.name),
			updateCmd,
			serveCmd,
			b.localDeps).
			WithAllowParallel(b.localAllowParallel)
		m = model.Manifest{Name: b.name, ResourceDependencies: rds}.WithDeployTarget(lt)
	} else {
		b.f.T().Fatalf("No deploy target specified: %s", b.name)
		return model.Manifest{}
	}
	m = m.WithTriggerMode(b.triggerMode)
	return m
}

type Fixture interface {
	T() testing.TB
	Path() string
	JoinPath(ps ...string) string
	MkdirAll(p string)
}
