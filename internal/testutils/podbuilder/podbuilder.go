package podbuilder

import (
	"fmt"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/pkg/apis"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/model"
)

const fakeContainerID = container.ID("myTestContainer")

func FakeContainerID() container.ID {
	return FakeContainerIDAtIndex(0)
}

func FakeContainerIDAtIndex(index int) container.ID {
	indexSuffix := ""
	if index != 0 {
		indexSuffix = fmt.Sprintf("-%d", index)
	}
	return container.ID(fmt.Sprintf("%s%s", fakeContainerID, indexSuffix))
}

func FakeContainerIDSet(size int) map[container.ID]bool {
	result := container.NewIDSet()
	for i := 0; i < size; i++ {
		result[FakeContainerIDAtIndex(i)] = true
	}
	return result
}

// Builds Pod objects for testing
//
// The pod model should be internally well-formed (e.g., the containers
// in the PodSpec object should match the containers in the PodStatus object).
//
// The pod model should also be consistent with the Manifest (e.g., if the Manifest
// specifies a Deployment with labels in a PodTemplateSpec, then any Pods should also
// have those labels).
//
// The PodBuilder is responsible for making sure we create well-formed Pods for
// testing. Tests should never modify the pod directly, but instead use the PodBuilder
// methods to ensure that the pod is consistent.
type PodBuilder struct {
	t        testing.TB
	manifest model.Manifest

	podUID          types.UID
	podName         string
	phase           string
	creationTime    time.Time
	deletionTime    time.Time
	restartCount    int
	extraPodLabels  map[string]string
	deploymentUID   types.UID
	resourceVersion string
	unknownOwner    bool

	// keyed by container index -- i.e. the first container will have image: imageRefs[0] and ID: cIDs[0], etc.
	// If there's no entry at index i, we'll use a dummy value.
	imageRefs map[int]string
	cIDs      map[int]string
	cReady    map[int]bool

	setPodTemplateSpecHash bool
	podTemplateSpecHash    k8s.PodTemplateSpecHash
}

func New(t testing.TB, manifest model.Manifest) PodBuilder {
	return PodBuilder{
		t:                      t,
		manifest:               manifest,
		creationTime:           time.Now(),
		imageRefs:              make(map[int]string),
		cIDs:                   make(map[int]string),
		cReady:                 make(map[int]bool),
		extraPodLabels:         make(map[string]string),
		setPodTemplateSpecHash: true,
	}
}

// Remove the owner reference. Useful for testing pod watching when
// the owner chain is broken (as in some CRDs).
func (b PodBuilder) WithUnknownOwner() PodBuilder {
	b.unknownOwner = true
	return b
}

func (b PodBuilder) WithPodLabel(key, val string) PodBuilder {
	b.extraPodLabels[key] = val
	return b
}

func (b PodBuilder) ManifestName() model.ManifestName {
	return b.manifest.Name
}

func (b PodBuilder) WithTemplateSpecHash(s k8s.PodTemplateSpecHash) PodBuilder {
	b.podTemplateSpecHash = s
	return b
}

func (b PodBuilder) WithNoTemplateSpecHash() PodBuilder {
	b.setPodTemplateSpecHash = false
	return b
}

func (b PodBuilder) RestartCount() int {
	return b.restartCount
}

func (b PodBuilder) WithRestartCount(restartCount int) PodBuilder {
	b.restartCount = restartCount
	return b
}

func (b PodBuilder) WithResourceVersion(rv string) PodBuilder {
	b.resourceVersion = rv
	return b
}

func (b PodBuilder) WithPodUID(uid types.UID) PodBuilder {
	b.podUID = uid
	return b
}

func (b PodBuilder) WithPodName(name string) PodBuilder {
	msgs := validation.NameIsDNSSubdomain(name, false)
	if len(msgs) != 0 {
		b.t.Fatalf("pod id %q is invalid: %s", name, msgs)
	}
	b.podName = name
	return b
}

func (b PodBuilder) WithPhase(phase string) PodBuilder {
	b.phase = phase
	return b
}

func (b PodBuilder) WithImage(image string) PodBuilder {
	return b.WithImageAtIndex(image, 0)
}

func (b PodBuilder) WithImageAtIndex(image string, index int) PodBuilder {
	b.imageRefs[index] = image
	return b
}

func (b PodBuilder) WithContainerID(cID container.ID) PodBuilder {
	return b.WithContainerIDAtIndex(cID, 0)
}

func (b PodBuilder) WithContainerIDAtIndex(cID container.ID, index int) PodBuilder {
	if cID == "" {
		b.cIDs[index] = ""
	} else {
		b.cIDs[index] = fmt.Sprintf("%s%s", k8s.ContainerIDPrefix, cID)
	}
	return b
}

func (b PodBuilder) WithContainerReady(ready bool) PodBuilder {
	return b.WithContainerReadyAtIndex(ready, 0)
}

func (b PodBuilder) WithContainerReadyAtIndex(ready bool, index int) PodBuilder {
	b.cReady[index] = ready
	return b
}

func (b PodBuilder) WithCreationTime(creationTime time.Time) PodBuilder {
	b.creationTime = creationTime
	return b
}

func (b PodBuilder) WithDeletionTime(deletionTime time.Time) PodBuilder {
	b.deletionTime = deletionTime
	return b
}

func (b PodBuilder) PodName() k8s.PodID {
	if b.podName != "" {
		return k8s.PodID(b.podName)
	}
	return k8s.PodID(fmt.Sprintf("%s-fakePodID", b.manifest.Name))
}

func (b PodBuilder) PodUID() types.UID {
	if b.podUID != "" {
		return b.podUID
	}
	return types.UID(fmt.Sprintf("%s-fakeUID", b.PodName()))
}

func (b PodBuilder) WithDeploymentUID(deploymentUID types.UID) PodBuilder {
	b.deploymentUID = deploymentUID
	return b
}

func (b PodBuilder) buildReplicaSetName() string {
	return fmt.Sprintf("%s-replicaset", b.manifest.Name)
}

func (b PodBuilder) buildReplicaSetUID() types.UID {
	if b.deploymentUID != "" {
		// if there's a custom Deployment UID, use that as the base for the ReplicaSet since
		// Deployments create ReplicaSets, and otherwise we can mix up this ReplicaSet with
		// the "default" Deployment since they'd have the same UID
		return types.UID(fmt.Sprintf("%s-rs-fakeUID", b.deploymentUID))
	}
	return types.UID(fmt.Sprintf("%s-fakeUID", b.buildReplicaSetName()))
}

func (b PodBuilder) buildDeploymentName() string {
	return fmt.Sprintf("%s-deployment", b.manifest.Name)
}

func (b PodBuilder) DeploymentUID() types.UID {
	if b.deploymentUID != "" {
		return b.deploymentUID
	}
	return types.UID(fmt.Sprintf("%s-fakeUID", b.buildDeploymentName()))
}

func (b PodBuilder) buildDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.buildDeploymentName(),
			Namespace: k8s.DefaultNamespace.String(),
			Labels:    k8s.NewTiltLabelMap(),
			UID:       b.DeploymentUID(),
		},
	}
}

func (b PodBuilder) buildReplicaSet() *appsv1.ReplicaSet {
	dep := b.buildDeployment()
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.buildReplicaSetName(),
			Namespace: k8s.DefaultNamespace.String(),
			UID:       b.buildReplicaSetUID(),
			Labels:    k8s.NewTiltLabelMap(),
			OwnerReferences: []metav1.OwnerReference{
				k8s.RuntimeObjToOwnerRef(dep),
			},
		},
	}
}

func (b PodBuilder) buildCreationTime() metav1.Time {
	return apis.NewTime(b.creationTime)
}

func (b PodBuilder) buildDeletionTime() *metav1.Time {
	if !b.deletionTime.IsZero() {
		v := apis.NewTime(b.deletionTime)
		return &v
	}
	return nil
}

func (b PodBuilder) buildLabels(tSpec *v1.PodTemplateSpec) map[string]string {
	labels := k8s.NewTiltLabelMap()
	for k, v := range tSpec.Labels {
		labels[k] = v
	}
	for k, v := range b.extraPodLabels {
		labels[k] = v
	}

	if b.setPodTemplateSpecHash {
		podTemplateSpecHash := b.podTemplateSpecHash
		if podTemplateSpecHash == "" {
			var err error
			podTemplateSpecHash, err = k8s.HashPodTemplateSpec(tSpec)
			if err != nil {
				panic(fmt.Sprintf("error computing pod template spec hash: %v", err))
			}
		}
		labels[k8s.TiltPodTemplateHashLabel] = string(podTemplateSpecHash)
	}

	return labels
}

func (b PodBuilder) buildImage(imageSpec string, index int) string {
	image, ok := b.imageRefs[index]
	if ok {
		return image
	}

	imageSpecRef := container.MustParseNamed(imageSpec)

	// Use the pod ID as the image tag. This is kind of weird, but gets at the semantics
	// we want (e.g., a new pod ID indicates that this is a new build).
	// Tests that don't want this behavior should replace the image with setImage(pod, imageName)
	return fmt.Sprintf("%s:%s", imageSpecRef.Name(), b.PodName())
}

func (b PodBuilder) buildContainerID(index int) string {
	cID, ok := b.cIDs[index]
	if ok {
		return cID
	}

	return fmt.Sprintf("%s%s", k8s.ContainerIDPrefix, FakeContainerIDAtIndex(index))
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
		restartCount := 0
		if i == 0 {
			restartCount = b.restartCount
		}
		ready, ok := b.cReady[i]
		// if not specified, default to true
		ready = !ok || ready

		state := v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: b.buildCreationTime(),
			},
		}

		result[i] = v1.ContainerStatus{
			Name:         cSpec.Name,
			Image:        b.buildImage(cSpec.Image, i),
			Ready:        ready,
			State:        state,
			ContainerID:  b.buildContainerID(i),
			RestartCount: int32(restartCount),
		}
	}
	return result
}

func (b PodBuilder) validateImageRefs(numContainers int) {
	for index, img := range b.imageRefs {
		if index >= numContainers {
			b.t.Fatalf("Image %q specified at index %d. Pod only has %d containers", img, index, numContainers)
		}
	}
}

func (b PodBuilder) validateContainerIDs(numContainers int) {
	for index, cID := range b.cIDs {
		if index >= numContainers {
			b.t.Fatalf("Container ID %q specified at index %d. Pod only has %d containers", cID, index, numContainers)
		}
	}
}

type PodObjectTree []k8s.K8sEntity

func (p PodObjectTree) Pod() k8s.K8sEntity {
	return p[0]
}

func (p PodObjectTree) ReplicaSet() k8s.K8sEntity {
	return p[1]
}

func (p PodObjectTree) Deployment() k8s.K8sEntity {
	return p[2]
}

// Simulates a Pod -> ReplicaSet -> Deployment ref tree
func (b PodBuilder) ObjectTreeEntities() PodObjectTree {
	pod := b.Build()
	rs := b.buildReplicaSet()
	dep := b.buildDeployment()
	return PodObjectTree{
		k8s.NewK8sEntity(pod),
		k8s.NewK8sEntity(rs),
		k8s.NewK8sEntity(dep),
	}
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
	numContainers := len(spec.Containers)
	b.validateImageRefs(numContainers)
	b.validateContainerIDs(numContainers)

	// Generate buildLabels from the incoming pod spec, before we've modified it,
	// so that it matches the spec we generate from the manifest itself.
	// Can override this behavior by setting b.PodTemplateSpecHash (or
	// by setting b.setPodTemplateSpecHash = false )
	labels := b.buildLabels(tSpec)

	for i, container := range spec.Containers {
		container.Image = b.buildImage(container.Image, i)
		spec.Containers[i] = container
	}

	ownerRefs := []metav1.OwnerReference{
		k8s.RuntimeObjToOwnerRef(b.buildReplicaSet()),
	}
	if b.unknownOwner {
		ownerRefs = nil
	}

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              string(b.PodName()),
			Namespace:         k8s.DefaultNamespace.String(),
			CreationTimestamp: b.buildCreationTime(),
			DeletionTimestamp: b.buildDeletionTime(),
			Labels:            labels,
			UID:               b.PodUID(),
			OwnerReferences:   ownerRefs,
			ResourceVersion:   b.resourceVersion,
		},
		Spec: spec,
		Status: v1.PodStatus{
			Phase:             b.buildPhase(),
			ContainerStatuses: b.buildContainerStatuses(spec),
		},
	}
}

func parseYAMLFromManifest(m model.Manifest) ([]k8s.K8sEntity, error) {
	return k8s.ParseYAMLFromString(m.K8sTarget().YAML)
}
