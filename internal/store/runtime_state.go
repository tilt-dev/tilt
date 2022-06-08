package store

import (
	"fmt"
	"net/url"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/model"
)

type RuntimeState interface {
	RuntimeState()

	// There are two types of resource dependencies:
	// - servers (Deployments) where what's important is that the server is running
	// - tasks (Jobs, local resources) where what's important is that the job completed
	// Currently, we don't try to distinguish between these two cases.
	//
	// In the future, it might make sense to check "IsBlocking()" or something,
	// and alter the behavior based on whether the underlying resource is a server
	// or a task.
	HasEverBeenReadyOrSucceeded() bool

	RuntimeStatus() v1alpha1.RuntimeStatus

	// If the runtime status is in Error mode,
	// RuntimeStatusError() should report a reason.
	RuntimeStatusError() error
}

type LocalRuntimeState struct {
	CmdName                  string
	Status                   v1alpha1.RuntimeStatus
	PID                      int
	StartTime                time.Time
	FinishTime               time.Time
	SpanID                   model.LogSpanID
	LastReadyOrSucceededTime time.Time
	Ready                    bool
}

var _ RuntimeState = LocalRuntimeState{}

func (LocalRuntimeState) RuntimeState() {}

func (l LocalRuntimeState) RuntimeStatus() v1alpha1.RuntimeStatus {
	status := l.Status
	if status == "" {
		status = v1alpha1.RuntimeStatusUnknown
	}
	return status
}

func (l LocalRuntimeState) RuntimeStatusError() error {
	status := l.RuntimeStatus()
	if status != v1alpha1.RuntimeStatusError {
		return nil
	}
	return fmt.Errorf("Process %d exited with non-zero status", l.PID)
}

func (l LocalRuntimeState) HasEverBeenReadyOrSucceeded() bool {
	return !l.LastReadyOrSucceededTime.IsZero()
}

type K8sRuntimeState struct {
	LBs map[k8s.ServiceName]*url.URL

	ApplyFilter *k8sconv.KubernetesApplyFilter

	// This must match the FilteredPods field of k8sconv.KubernetesResource
	FilteredPods []v1alpha1.Pod

	// Conditions from the apply operation; must match the Conditions field
	// from k8sconv.KubernetesResource::ApplyStatus.
	Conditions []metav1.Condition

	LastReadyOrSucceededTime    time.Time
	HasEverDeployedSuccessfully bool

	UpdateStartTime map[k8s.PodID]time.Time

	PodReadinessMode model.PodReadinessMode
}

func (K8sRuntimeState) RuntimeState() {}

var _ RuntimeState = K8sRuntimeState{}

func NewK8sRuntimeStateWithPods(m model.Manifest, pods ...v1alpha1.Pod) K8sRuntimeState {
	state := NewK8sRuntimeState(m)
	state.FilteredPods = pods
	state.HasEverDeployedSuccessfully = len(pods) > 0
	return state
}

func NewK8sRuntimeState(m model.Manifest) K8sRuntimeState {
	return K8sRuntimeState{
		PodReadinessMode: m.PodReadinessMode(),
		LBs:              make(map[k8s.ServiceName]*url.URL),
		UpdateStartTime:  make(map[k8s.PodID]time.Time),
	}
}

func (s K8sRuntimeState) RuntimeStatusError() error {
	status := s.RuntimeStatus()
	if status != v1alpha1.RuntimeStatusError {
		return nil
	}
	pod := s.MostRecentPod()
	return fmt.Errorf("Pod %s in error state: %s", pod.Name, pod.Status)
}

func (s K8sRuntimeState) RuntimeStatus() v1alpha1.RuntimeStatus {
	if !s.HasEverDeployedSuccessfully {
		return v1alpha1.RuntimeStatusPending
	}

	if s.PodReadinessMode == model.PodReadinessIgnore {
		return v1alpha1.RuntimeStatusOK
	}

	// if the apply indicated that the Job had already completed, we can skip
	// inspecting the Pods, which avoids issues in the event that the Job's
	// Pod was GC'd
	if meta.IsStatusConditionTrue(s.Conditions, v1alpha1.ApplyConditionJobComplete) {
		return v1alpha1.RuntimeStatusOK
	}

	pod := s.MostRecentPod()
	switch v1.PodPhase(pod.Phase) {
	case v1.PodRunning:
		if AllPodContainersReady(pod) && s.PodReadinessMode != model.PodReadinessSucceeded {
			return v1alpha1.RuntimeStatusOK
		}
		return v1alpha1.RuntimeStatusPending

	case v1.PodSucceeded:
		return v1alpha1.RuntimeStatusOK

	case v1.PodFailed:
		return v1alpha1.RuntimeStatusError
	}

	for _, c := range AllPodContainers(pod) {
		if k8sconv.ContainerStatusToRuntimeState(c) == v1alpha1.RuntimeStatusError {
			return v1alpha1.RuntimeStatusError
		}
	}

	return v1alpha1.RuntimeStatusPending
}

func (s K8sRuntimeState) HasEverBeenReadyOrSucceeded() bool {
	if !s.HasEverDeployedSuccessfully {
		return false
	}
	if s.PodReadinessMode == model.PodReadinessIgnore {
		return true
	}
	return !s.LastReadyOrSucceededTime.IsZero()
}

func (s K8sRuntimeState) PodLen() int {
	return len(s.FilteredPods)
}

func (s K8sRuntimeState) ContainsID(id k8s.PodID) bool {
	name := string(id)
	for _, pod := range s.FilteredPods {
		if pod.Name == name {
			return true
		}
	}
	return false
}

func (s K8sRuntimeState) GetPods() []v1alpha1.Pod {
	return s.FilteredPods
}

func (s K8sRuntimeState) EntityDisplayNames() []string {
	if s.ApplyFilter == nil {
		return nil
	}

	entities := make([]k8s.EntityMeta, len(s.ApplyFilter.DeployedRefs))
	for i := range s.ApplyFilter.DeployedRefs {
		entities[i] = objectRefMeta{s.ApplyFilter.DeployedRefs[i]}
	}

	// Use a min component count of 2 for computing names,
	// so that the resource type appears
	return k8s.UniqueNamesMeta(entities, 2)
}

type objectRefMeta struct {
	v1.ObjectReference
}

func (o objectRefMeta) Name() string {
	return o.ObjectReference.Name
}

func (o objectRefMeta) Namespace() k8s.Namespace {
	return k8s.Namespace(o.ObjectReference.Namespace)
}

func (o objectRefMeta) GVK() schema.GroupVersionKind {
	return o.ObjectReference.GroupVersionKind()
}

// Get the "most recent pod" from the K8sRuntimeState.
// For most users, we believe there will be only one pod per manifest.
// So most of this time, this will return the only pod.
// And in other cases, it will return a reasonable, consistent default.
func (s K8sRuntimeState) MostRecentPod() v1alpha1.Pod {
	return MostRecentPod(s.GetPods())
}

func AllPodContainers(p v1alpha1.Pod) []v1alpha1.Container {
	var result []v1alpha1.Container
	result = append(result, p.InitContainers...)
	result = append(result, p.Containers...)
	return result
}

func AllPodContainersReady(p v1alpha1.Pod) bool {
	if len(p.Containers) == 0 {
		return false
	}

	for _, c := range p.Containers {
		if !c.Ready {
			return false
		}
	}
	return true
}

func AllPodContainerRestarts(p v1alpha1.Pod) int32 {
	result := int32(0)
	for _, c := range p.Containers {
		result += c.Restarts
	}
	return result
}

func (s K8sRuntimeState) VisiblePodContainerRestarts(podID k8s.PodID) int32 {
	name := string(podID)
	for _, pod := range s.GetPods() {
		if pod.Name == name {
			return AllPodContainerRestarts(pod)
		}
	}
	return 0
}

func AllPodContainerPorts(p v1alpha1.Pod) []int32 {
	result := make([]int32, 0)
	for _, c := range p.Containers {
		result = append(result, c.Ports...)
	}
	return result
}

func MostRecentPod(list []v1alpha1.Pod) v1alpha1.Pod {
	bestPod := v1alpha1.Pod{}
	found := false

	for _, pod := range list {
		if !found || k8sconv.PodCompare(pod, bestPod) {
			bestPod = pod
			found = true
		}
	}

	return bestPod
}
