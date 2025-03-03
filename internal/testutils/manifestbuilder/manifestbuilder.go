package manifestbuilder

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cmdimage"
	"github.com/tilt-dev/tilt/internal/controllers/apis/dockerimage"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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
	k8sImageLocators   []v1alpha1.KubernetesImageLocator
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
	b.k8sImageLocators = append(b.k8sImageLocators, v1alpha1.KubernetesImageLocator{
		ObjectSelector: k8s.MustNameSelector(name).ToSpec(),
		Path:           path,
	})
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

func (b ManifestBuilder) WithLiveUpdate(lu v1alpha1.LiveUpdateSpec) ManifestBuilder {
	return b.WithLiveUpdateAtIndex(lu, 0)
}

func (b ManifestBuilder) WithLiveUpdateAtIndex(lu v1alpha1.LiveUpdateSpec, index int) ManifestBuilder {
	if len(b.iTargets) <= index {
		b.f.T().Fatalf("WithLiveUpdateAtIndex: index %d out of range -- (manifestBuilder has %d image targets)", index, len(b.iTargets))
	}

	iTarg := b.iTargets[index]
	iTarg.LiveUpdateSpec = lu
	if !liveupdate.IsEmptySpec(lu) {
		iTarg.LiveUpdateName = liveupdate.GetName(b.name, iTarg.ID())
	}
	b.iTargets[index] = iTarg
	return b
}

func (b ManifestBuilder) WithResourceDeps(deps ...string) ManifestBuilder {
	b.resourceDeps = deps
	return b
}

func (b ManifestBuilder) Build() model.Manifest {
	var m model.Manifest

	// Adjust images to use their API server object names, when appropriate.
	// Currently,
	for index, iTarget := range b.iTargets {
		if iTarget.IsDockerBuild() {
			iTarget.DockerImageName = dockerimage.GetName(b.name, iTarget.ID())
		} else if iTarget.IsCustomBuild() {
			iTarget.CmdImageName = cmdimage.GetName(b.name, iTarget.ID())
		}

		if len(b.dcConfigPaths) != 0 {
			if iTarget.IsDockerBuild() {
				dbi := iTarget.DockerBuildInfo()
				dbi.DockerImageSpec.Cluster = v1alpha1.ClusterNameDocker
				iTarget.BuildDetails = dbi
			} else if iTarget.IsCustomBuild() {
				cbi := iTarget.CustomBuildInfo()
				cbi.CmdImageSpec.Cluster = v1alpha1.ClusterNameDocker
				iTarget.BuildDetails = cbi
			}
		}

		if liveupdate.IsEmptySpec(iTarget.LiveUpdateSpec) {
			iTarget.LiveUpdateReconciler = false
		} else {
			iTarget.LiveUpdateReconciler = true
		}
		b.iTargets[index] = iTarget
	}

	var rds []model.ManifestName
	for _, dep := range b.resourceDeps {
		rds = append(rds, model.ManifestName(dep))
	}

	if b.k8sYAML != "" {
		k8sTarget := k8s.MustTarget(model.TargetName(b.name), b.k8sYAML)
		k8sTarget.KubernetesApplySpec.KubernetesDiscoveryTemplateSpec = &v1alpha1.KubernetesDiscoveryTemplateSpec{
			ExtraSelectors: k8s.SetsAsLabelSelectors(b.k8sPodSelectors),
		}
		k8sTarget.ImageLocators = append(k8sTarget.ImageLocators, b.k8sImageLocators...)
		k8sTarget.PodReadinessMode = b.k8sPodReadiness

		m = assembleK8s(
			model.Manifest{Name: b.name, ResourceDependencies: rds},
			k8sTarget,
			b.iTargets...)
	} else if len(b.dcConfigPaths) > 0 {
		m = assembleDC(
			model.Manifest{Name: b.name, ResourceDependencies: rds},
			model.DockerComposeTarget{
				Spec: v1alpha1.DockerComposeServiceSpec{
					Service: string(b.name),
					Project: v1alpha1.DockerComposeProject{
						ConfigPaths: b.dcConfigPaths,
					},
				},
				Name: model.TargetName(b.name),
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

		m = m.WithDisableSource(&v1alpha1.DisableSource{
			ConfigMap: &v1alpha1.ConfigMapDisableSource{
				Name: fmt.Sprintf("%s-disable", b.name),
				Key:  "isDisabled",
			},
		})
	} else {
		b.f.T().Fatalf("No deploy target specified: %s", b.name)
		return model.Manifest{}
	}
	m = m.WithTriggerMode(b.triggerMode)

	err := model.InferImageProperties([]model.Manifest{m})
	require.NoError(b.f.T(), err)

	err = m.InferLiveUpdateSelectors()
	require.NoError(b.f.T(), err)
	return m
}

type Fixture interface {
	T() testing.TB
	Path() string
	JoinPath(ps ...string) string
	MkdirAll(p string)
}
