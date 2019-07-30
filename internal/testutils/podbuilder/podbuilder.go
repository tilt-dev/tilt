package podbuilder

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

const FakeDeployID = model.DeployID(1234567890)
const FakeContainerID = "myTestContainer"

// Builds Pod objects for testing
//
// Ensures that any PodStatus objects we create are consistent with the Pod in
// the PodTemplateSpec in the manifest.
type PodBuilder struct {
	t        testing.TB
	manifest model.Manifest

	podID        string
	phase        string
	creationTime time.Time
	deployID     model.DeployID

	// keyed by container index
	imageRefs map[int]string
	cIDs      map[int]string
}

func New(t testing.TB, manifest model.Manifest) PodBuilder {
	return PodBuilder{
		t:         t,
		manifest:  manifest,
		imageRefs: make(map[int]string),
		cIDs:      make(map[int]string),
	}
}

func (b PodBuilder) WithPodID(podID string) PodBuilder {
	msgs := validation.NameIsDNSSubdomain(podID, false)
	if len(msgs) != 0 {
		b.t.Fatalf("pod id %q is invalid: %s", podID, msgs)
	}
	b.podID = podID
	return b
}

func (b PodBuilder) WithPhase(phase string) PodBuilder {
	b.phase = phase
	return b
}

func (b PodBuilder) WithImage(image string, index int) PodBuilder {
	b.imageRefs[index] = image
	return b
}

func (b PodBuilder) WithContainerID(cID string, index int) PodBuilder {
	if cID == "" {
		b.cIDs[index] = ""
	} else {
		b.cIDs[index] = fmt.Sprintf("%s%s", k8s.ContainerIDPrefix, cID)
	}
	return b
}

func (b PodBuilder) WithCreationTime(creationTime time.Time) PodBuilder {
	b.creationTime = creationTime
	return b
}

func (b PodBuilder) WithDeployID(deployID model.DeployID) PodBuilder {
	b.deployID = deployID
	return b
}

func (b PodBuilder) buildPodID() string {
	if b.podID != "" {
		return b.podID
	}
	return "fakePodID"
}

func (b PodBuilder) buildCreationTime() metav1.Time {
	if !b.creationTime.IsZero() {
		return metav1.Time{Time: b.creationTime}
	}
	return metav1.Time{Time: time.Now()}
}

func (b PodBuilder) buildLabels(tSpec *v1.PodTemplateSpec) map[string]string {
	deployID := b.deployID
	if deployID.Empty() {
		deployID = FakeDeployID
	}
	labels := map[string]string{
		k8s.ManifestNameLabel: b.manifest.Name.String(),
		k8s.TiltDeployIDLabel: deployID.String(),
	}

	for k, v := range tSpec.Labels {
		labels[k] = v
	}
	return labels
}

func (b PodBuilder) buildImage(index int) string {
	image, ok := b.imageRefs[index]
	if ok {
		return image
	}

	indexSuffix := ""
	if index != 0 {
		indexSuffix = fmt.Sprintf("-%d", index)
	}

	// Use the pod ID as the image tag. This is kind of weird, but gets at the semantics
	// we want (e.g., a new pod ID indicates that this is a new build).
	// Tests that don't want this behavior should replace the image with setImage(pod, imageName)
	return fmt.Sprintf("%s%s:%s", imageNameForManifest(b.manifest.Name.String()).String(), indexSuffix, b.buildPodID())
}

func (b PodBuilder) buildContainerID(index int) string {
	cID, ok := b.cIDs[index]
	if ok {
		return cID
	}

	indexSuffix := ""
	if index != 0 {
		indexSuffix = fmt.Sprintf("-%d", index)
	}
	return fmt.Sprintf("%s%s%s", k8s.ContainerIDPrefix, FakeContainerID, indexSuffix)
}

func (b PodBuilder) buildPhase() v1.PodPhase {
	if b.phase == "" {
		return v1.PodPhase("Running")
	}
	return v1.PodPhase(b.phase)
}

func (b PodBuilder) buildContainerStatuses(spec v1.PodSpec) []v1.ContainerStatus {
	result := make([]v1.ContainerStatus, len(spec.Containers))
	for i, cSpec := range spec.Containers {
		result[i] = v1.ContainerStatus{
			Name:        cSpec.Name,
			Image:       b.buildImage(i),
			Ready:       true,
			ContainerID: b.buildContainerID(i),
		}
	}
	return result
}

func (b PodBuilder) Build() *v1.Pod {
	entities, err := parseYAMLFromManifest(b.manifest)
	if err != nil {
		b.t.Fatal(fmt.Errorf("PodBuilder YAML parser: %v", err))
	}

	tSpecs, err := k8s.ExtractPodTemplateSpec(entities)
	if err != nil {
		b.t.Fatal(fmt.Errorf("PodBuilder extract pod templates: %v", err))
	}

	if len(tSpecs) != 1 {
		b.t.Fatalf("PodBuilder only works with Manifests with exactly 1 PodTemplateSpec: %v", tSpecs)
	}
	tSpec := tSpecs[0]
	spec := tSpec.Spec
	for i, container := range spec.Containers {
		container.Image = b.buildImage(i)
		spec.Containers[i] = container
	}

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              b.buildPodID(),
			CreationTimestamp: b.buildCreationTime(),
			Labels:            b.buildLabels(tSpec),
		},
		Spec: spec,
		Status: v1.PodStatus{
			Phase:             b.buildPhase(),
			ContainerStatuses: b.buildContainerStatuses(spec),
		},
	}
}

func imageNameForManifest(manifestName string) reference.Named {
	return container.MustParseNamed(manifestName)
}

func parseYAMLFromManifest(m model.Manifest) ([]k8s.K8sEntity, error) {
	return k8s.ParseYAMLFromString(m.K8sTarget().YAML)
}
