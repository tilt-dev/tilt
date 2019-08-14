package store

import (
	"net/url"
	"time"

	"github.com/docker/distribution/reference"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/pkg/model"
)

type RuntimeState interface {
	RuntimeState()
}

type K8sRuntimeState struct {
	PodDeployID model.DeployID // Deploy that pods correspond to
	Pods        map[k8s.PodID]*Pod
	LBs         map[k8s.ServiceName]*url.URL
}

func (K8sRuntimeState) RuntimeState() {}

func NewK8sRuntimeState(deployID model.DeployID, pods ...Pod) K8sRuntimeState {
	podMap := make(map[k8s.PodID]*Pod, len(pods))
	for _, pod := range pods {
		p := pod
		podMap[p.PodID] = &p
	}
	return K8sRuntimeState{
		PodDeployID: deployID,
		Pods:        podMap,
		LBs:         make(map[k8s.ServiceName]*url.URL),
	}
}

func (s K8sRuntimeState) PodLen() int {
	return len(s.Pods)
}

func (s K8sRuntimeState) ContainsID(id k8s.PodID) bool {
	_, ok := s.Pods[id]
	return ok
}

func (s K8sRuntimeState) PodList() []Pod {
	pods := make([]Pod, 0, len(s.Pods))
	for _, pod := range s.Pods {
		pods = append(pods, *pod)
	}
	return pods
}

// Get the "most recent pod" from the K8sRuntimeState.
// For most users, we believe there will be only one pod per manifest.
// So most of this time, this will return the only pod.
// And in other cases, it will return a reasonable, consistent default.
func (s K8sRuntimeState) MostRecentPod() Pod {
	bestPod := Pod{}
	found := false

	for _, v := range s.Pods {
		if !found || v.isAfter(bestPod) {
			bestPod = *v
			found = true
		}
	}

	return bestPod
}

type Pod struct {
	PodID     k8s.PodID
	Namespace k8s.Namespace
	StartedAt time.Time
	Status    string
	Phase     v1.PodPhase

	// Error messages from the pod state if it's in an error state.
	StatusMessages []string

	// Set when we get ready to replace a pod. We may do the update in-place.
	UpdateStartTime time.Time

	// If a pod is being deleted, Kubernetes marks it as Running
	// until it actually gets removed.
	Deleting bool

	HasSynclet bool

	// The log for the currently active pod, if any
	CurrentLog model.Log `testdiff:"ignore"`

	Containers []Container

	// We want to show the user # of restarts since pod has been running current code,
	// i.e. OldRestarts - Total Restarts
	OldRestarts int // # times the pod restarted when it was running old code
}

type Container struct {
	Name     container.Name
	ID       container.ID
	Ports    []int32
	Ready    bool
	ImageRef reference.Named
	Restarts int
}

func (c Container) Empty() bool {
	return c.Name == "" && c.ID == ""
}

func (p Pod) Empty() bool {
	return p.PodID == ""
}

// A stable sort order for pods.
func (p Pod) isAfter(p2 Pod) bool {
	if p.StartedAt.After(p2.StartedAt) {
		return true
	} else if p2.StartedAt.After(p.StartedAt) {
		return false
	}
	return p.PodID > p2.PodID
}

func (p Pod) Log() model.Log {
	return p.CurrentLog
}

func (p Pod) AllContainerPorts() []int32 {
	result := make([]int32, 0)
	for _, c := range p.Containers {
		result = append(result, c.Ports...)
	}
	return result
}

func (p Pod) AllContainersReady() bool {
	for _, c := range p.Containers {
		if !c.Ready {
			return false
		}
	}
	return true
}

func (p Pod) AllContainerRestarts() int {
	result := 0
	for _, c := range p.Containers {
		result += c.Restarts
	}
	return result
}
