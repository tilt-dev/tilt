package store

import (
	"fmt"
	"net/url"
	"time"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

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
	return l.Status
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
	// The ancestor that we match pods against to associate them with this manifest.
	// If we deployed Pod YAML, this will be the Pod UID.
	// In many cases, this will be a Deployment UID.
	PodAncestorUID types.UID

	Pods PodSet
	LBs  map[k8s.ServiceName]*url.URL

	ApplyFilter *k8sconv.KubernetesApplyFilter

	LastReadyOrSucceededTime    time.Time
	HasEverDeployedSuccessfully bool

	UpdateStartTime map[k8s.PodID]time.Time

	PodReadinessMode model.PodReadinessMode

	// BaselineRestarts is used as a floor for container restarts to avoid alerting on restarts
	// that happened either before Tilt started or before a Live Update change.
	BaselineRestarts map[k8s.PodID]int32
}

func (K8sRuntimeState) RuntimeState() {}

var _ RuntimeState = K8sRuntimeState{}

func NewK8sRuntimeStateWithPods(m model.Manifest, pods ...v1alpha1.Pod) K8sRuntimeState {
	state := NewK8sRuntimeState(m)
	for _, pod := range pods {
		p := pod
		state.Pods[k8s.PodID(p.Name)] = &p
	}
	state.HasEverDeployedSuccessfully = len(pods) > 0
	return state
}

func NewK8sRuntimeState(m model.Manifest) K8sRuntimeState {
	return K8sRuntimeState{
		PodReadinessMode: m.PodReadinessMode(),
		Pods:             PodSet{},
		LBs:              make(map[k8s.ServiceName]*url.URL),
		UpdateStartTime:  make(map[k8s.PodID]time.Time),
		BaselineRestarts: make(map[k8s.PodID]int32),
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
	return len(s.Pods)
}

func (s K8sRuntimeState) ContainsID(id k8s.PodID) bool {
	_, ok := s.Pods[id]
	return ok
}

func (s K8sRuntimeState) PodList() []v1alpha1.Pod {
	pods := make([]v1alpha1.Pod, 0, len(s.Pods))
	for _, pod := range s.Pods {
		pods = append(pods, *pod)
	}
	return pods
}

// Get the "most recent pod" from the K8sRuntimeState.
// For most users, we believe there will be only one pod per manifest.
// So most of this time, this will return the only pod.
// And in other cases, it will return a reasonable, consistent default.
func (s K8sRuntimeState) MostRecentPod() v1alpha1.Pod {
	return s.Pods.MostRecentPod()
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
	p := s.Pods[podID]
	if p == nil {
		return 0
	}
	return AllPodContainerRestarts(*p) - s.BaselineRestarts[podID]
}

func AllPodContainerPorts(p v1alpha1.Pod) []int32 {
	result := make([]int32, 0)
	for _, c := range p.Containers {
		result = append(result, c.Ports...)
	}
	return result
}

type PodSet map[k8s.PodID]*v1alpha1.Pod

func (ps PodSet) MostRecentPod() v1alpha1.Pod {
	bestPod := v1alpha1.Pod{}
	found := false

	for _, v := range ps {
		if !found || k8sconv.PodCompare(*v, bestPod) {
			bestPod = *v
			found = true
		}
	}

	return bestPod
}

func (ps PodSet) Filter(filter func(pod *v1alpha1.Pod) bool) PodSet {
	newSet := PodSet{}
	for k, v := range ps {
		if filter(v) {
			newSet[k] = v
		}
	}
	return newSet
}
