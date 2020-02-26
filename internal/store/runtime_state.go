package store

import (
	"net/url"
	"time"

	"github.com/docker/distribution/reference"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/pkg/model"
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
}

type LocalRuntimeState struct {
	Status                  model.RuntimeStatus
	HasSucceededAtLeastOnce bool
	PID                     int
	SpanID                  model.LogSpanID
}

func (LocalRuntimeState) RuntimeState() {}

func (l LocalRuntimeState) HasEverBeenReadyOrSucceeded() bool {
	return l.HasSucceededAtLeastOnce
}

var _ RuntimeState = LocalRuntimeState{}

type K8sRuntimeState struct {
	// The ancestor that we match pods against to associate them with this manifest.
	// If we deployed Pod YAML, this will be the Pod UID.
	// In many cases, this will be a Deployment UID.
	PodAncestorUID types.UID

	Pods                           map[k8s.PodID]*Pod
	LBs                            map[k8s.ServiceName]*url.URL
	DeployedUIDSet                 UIDSet                 // for the most recent successful deploy
	DeployedPodTemplateSpecHashSet PodTemplateSpecHashSet // for the most recent successful deploy

	LastReadyOrSucceededTime time.Time
}

func (K8sRuntimeState) RuntimeState() {}

var _ RuntimeState = K8sRuntimeState{}

func NewK8sRuntimeState(pods ...Pod) K8sRuntimeState {
	podMap := make(map[k8s.PodID]*Pod, len(pods))
	for _, pod := range pods {
		p := pod
		podMap[p.PodID] = &p
	}
	return K8sRuntimeState{
		Pods:                           podMap,
		LBs:                            make(map[k8s.ServiceName]*url.URL),
		DeployedUIDSet:                 NewUIDSet(),
		DeployedPodTemplateSpecHashSet: NewPodTemplateSpecHashSet(),
	}
}

func (s K8sRuntimeState) HasEverBeenReadyOrSucceeded() bool {
	return !s.LastReadyOrSucceededTime.IsZero()
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

func (s K8sRuntimeState) HasOKPodTemplateSpecHash(pod *v1.Pod) bool {
	// if it doesn't have a label, just let it through - maybe it's from a CRD w/ no pod template spec
	hash, ok := pod.Labels[k8s.TiltPodTemplateHashLabel]
	if !ok {
		return true
	}

	return s.DeployedPodTemplateSpecHashSet.Contains(k8s.PodTemplateSpecHash(hash))
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

	Containers []Container

	// We want to show the user # of restarts since some baseline time
	// i.e. Total Restarts - BaselineRestarts
	BaselineRestarts int

	Conditions []v1.PodCondition

	SpanID model.LogSpanID
}

type Container struct {
	Name     container.Name
	ID       container.ID
	Ports    []int32
	Ready    bool
	Running  bool
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

func (p Pod) AllContainerPorts() []int32 {
	result := make([]int32, 0)
	for _, c := range p.Containers {
		result = append(result, c.Ports...)
	}
	return result
}

func (p Pod) AllContainersReady() bool {
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

func (p Pod) VisibleContainerRestarts() int {
	return p.AllContainerRestarts() - p.BaselineRestarts
}

func (p Pod) AllContainerRestarts() int {
	result := 0
	for _, c := range p.Containers {
		result += c.Restarts
	}
	return result
}

type UIDSet map[types.UID]bool

func NewUIDSet() UIDSet {
	return make(map[types.UID]bool)
}

func (s UIDSet) Add(uids ...types.UID) {
	for _, uid := range uids {
		s[uid] = true
	}
}

func (s UIDSet) Contains(uid types.UID) bool {
	return s[uid]
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
