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

	RuntimeStatus() model.RuntimeStatus

	// If the runtime status is in Error mode,
	// RuntimeStatusError() should report a reason.
	RuntimeStatusError() error
}

type LocalRuntimeState struct {
	CmdName                  string
	Status                   model.RuntimeStatus
	PID                      int
	StartTime                time.Time
	FinishTime               time.Time
	SpanID                   model.LogSpanID
	LastReadyOrSucceededTime time.Time
	Ready                    bool
}

var _ RuntimeState = LocalRuntimeState{}

func (LocalRuntimeState) RuntimeState() {}

func (l LocalRuntimeState) RuntimeStatus() model.RuntimeStatus {
	return l.Status
}

func (l LocalRuntimeState) RuntimeStatusError() error {
	status := l.RuntimeStatus()
	if status != model.RuntimeStatusError {
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

	Pods                           map[k8s.PodID]*v1alpha1.Pod
	LBs                            map[k8s.ServiceName]*url.URL
	DeployedEntities               k8s.ObjRefList         // for the most recent successful deploy
	DeployedPodTemplateSpecHashSet PodTemplateSpecHashSet // for the most recent successful deploy

	LastReadyOrSucceededTime    time.Time
	HasEverDeployedSuccessfully bool

	PodReadinessMode model.PodReadinessMode
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
		PodReadinessMode:               m.PodReadinessMode(),
		Pods:                           make(map[k8s.PodID]*v1alpha1.Pod),
		LBs:                            make(map[k8s.ServiceName]*url.URL),
		DeployedPodTemplateSpecHashSet: NewPodTemplateSpecHashSet(),
	}
}

func (s K8sRuntimeState) RuntimeStatusError() error {
	status := s.RuntimeStatus()
	if status != model.RuntimeStatusError {
		return nil
	}
	pod := s.MostRecentPod()
	return fmt.Errorf("Pod %s in error state: %s", pod.Name, pod.Status)
}

func (s K8sRuntimeState) RuntimeStatus() model.RuntimeStatus {
	if !s.HasEverDeployedSuccessfully {
		return model.RuntimeStatusPending
	}

	if s.PodReadinessMode == model.PodReadinessIgnore {
		return model.RuntimeStatusOK
	}

	pod := s.MostRecentPod()

	switch v1.PodPhase(pod.Phase) {
	case v1.PodRunning:
		if AllPodContainersReady(pod) {
			return model.RuntimeStatusOK
		}
		return model.RuntimeStatusPending

	case v1.PodSucceeded:
		return model.RuntimeStatusOK

	case v1.PodFailed:
		return model.RuntimeStatusError
	}

	for _, c := range AllPodContainers(pod) {
		if k8sconv.ContainerStatusToRuntimeState(c) == model.RuntimeStatusError {
			return model.RuntimeStatusError
		}
	}

	return model.RuntimeStatusPending
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
	bestPod := v1alpha1.Pod{}
	found := false

	for _, v := range s.Pods {
		if !found || podCompare(*v, bestPod) {
			bestPod = *v
			found = true
		}
	}

	return bestPod
}

func (s K8sRuntimeState) HasOKPodTemplateSpecHash(pod *v1alpha1.Pod) bool {
	// if it doesn't have a label, just let it through - maybe it's from a CRD w/ no pod template spec
	hash := k8s.PodTemplateSpecHash(pod.PodTemplateSpecHash)
	if hash == "" {
		return true
	}

	return s.DeployedPodTemplateSpecHashSet.Contains(hash)
}

func (s K8sRuntimeState) DeployedUIDSet() k8s.UIDSet {
	uids := k8s.NewUIDSet()
	for _, ref := range s.DeployedEntities {
		uids.Add(ref.UID)
	}
	return uids
}

// podCompare is a stable sort order for pods.
func podCompare(p1 v1alpha1.Pod, p2 v1alpha1.Pod) bool {
	if p1.CreatedAt.After(p2.CreatedAt.Time) {
		return true
	} else if p2.CreatedAt.After(p1.CreatedAt.Time) {
		return false
	}
	return p1.Name > p2.Name
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

func AllPodContainerRestarts(p v1alpha1.Pod) int {
	result := 0
	for _, c := range p.Containers {
		result += int(c.Restarts)
	}
	return result
}

func VisiblePodContainerRestarts(p v1alpha1.Pod) int {
	return AllPodContainerRestarts(p) - p.BaselineRestartCount
}

func AllPodContainerPorts(p v1alpha1.Pod) []int32 {
	result := make([]int32, 0)
	for _, c := range p.Containers {
		result = append(result, c.Ports...)
	}
	return result
}

type PodTemplateSpecHashSet map[k8s.PodTemplateSpecHash]bool

func NewPodTemplateSpecHashSet() PodTemplateSpecHashSet {
	return make(map[k8s.PodTemplateSpecHash]bool)
}

func (s PodTemplateSpecHashSet) Add(hashes ...k8s.PodTemplateSpecHash) {
	for _, hash := range hashes {
		s[hash] = true
	}
}

func (s PodTemplateSpecHashSet) Contains(hash k8s.PodTemplateSpecHash) bool {
	return s[hash]
}
