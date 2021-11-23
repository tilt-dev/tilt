package k8sconv

import (
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// A KubernetesResource exposes a high-level status that summarizes
// the Pods we care about in a KubernetesDiscovery.
//
// If we have a KubernetesApply, KubernetesResource will use that
// to narrow down the list of pods to only the pods we care about
// for the current Apply.
//
// KubernetesResource is intended to be a non-stateful object (i.e., it is
// immutable and its status can be inferred from the state of child
// objects.)
//
// Long-term, this may become an explicit API server object, but
// for now it's intended to provide an API-server compatible
// layer around KubernetesDiscovery + KubernetesApply.
type KubernetesResource struct {
	Discovery   *v1alpha1.KubernetesDiscovery
	ApplyStatus *v1alpha1.KubernetesApplyStatus

	// A set of properties we use to determine which pods in Discovery
	// belong to the current Apply.
	ApplyFilter *KubernetesApplyFilter

	// A set of pods that belong to the current Discovery
	// and the current ApplyStatus (if available).
	//
	// Excludes pods that are being deleted
	// or which belong to a previous apply.
	FilteredPods []v1alpha1.Pod
}

func NewKubernetesResource(discovery *v1alpha1.KubernetesDiscovery, status *v1alpha1.KubernetesApplyStatus) (*KubernetesResource, error) {
	var filter *KubernetesApplyFilter
	var err error
	if status != nil {
		filter, err = NewKubernetesApplyFilter(status)
		if err != nil {
			return nil, err
		}
	}

	var filteredPods []v1alpha1.Pod
	if discovery != nil {
		filteredPods = FilterPods(filter, discovery.Status.Pods)
	}

	return &KubernetesResource{
		Discovery:    discovery,
		ApplyStatus:  status,
		ApplyFilter:  filter,
		FilteredPods: filteredPods,
	}, nil
}

// Filter to determine whether a pod or resource belongs to the current
// KubernetesApply. Used to filter out pods from previous applys
// when looking at a KubernetesDiscovery object.
//
// Considered immutable once created.
type KubernetesApplyFilter struct {
	// DeployedRefs are references to the objects that we deployed to a Kubernetes cluster.
	DeployedRefs []v1.ObjectReference

	// Hashes of the pod template specs that we deployed to a Kubernetes cluster.
	PodTemplateSpecHashes []k8s.PodTemplateSpecHash
}

func NewKubernetesApplyFilter(status *v1alpha1.KubernetesApplyStatus) (*KubernetesApplyFilter, error) {
	deployed, err := k8s.ParseYAMLFromString(status.ResultYAML)
	if err != nil {
		return nil, err
	}
	deployed = k8s.SortedEntities(deployed)

	podTemplateSpecHashes := []k8s.PodTemplateSpecHash{}
	for _, entity := range deployed {
		if entity.UID() == "" {
			return nil, fmt.Errorf("Resource missing uid: %s", entity.Name())
		}
		hs, err := k8s.ReadPodTemplateSpecHashes(entity)
		if err != nil {
			return nil, errors.Wrap(err, "reading pod template spec hashes")
		}
		podTemplateSpecHashes = append(podTemplateSpecHashes, hs...)
	}
	return &KubernetesApplyFilter{
		DeployedRefs:          k8s.ToRefList(deployed),
		PodTemplateSpecHashes: podTemplateSpecHashes,
	}, nil
}

func ContainsHash(filter *KubernetesApplyFilter, hash k8s.PodTemplateSpecHash) bool {
	if filter == nil {
		return false
	}

	for _, h := range filter.PodTemplateSpecHashes {
		if h == hash {
			return true
		}
	}
	return false
}

func ContainsUID(filter *KubernetesApplyFilter, uid types.UID) bool {
	if filter == nil {
		return false
	}

	for _, ref := range filter.DeployedRefs {
		if ref.UID == uid {
			return true
		}
	}
	return false
}

// Checks to see if the given pod is allowed by the current filter.
func HasOKPodTemplateSpecHash(pod *v1alpha1.Pod, filter *KubernetesApplyFilter) bool {
	// if it doesn't have a label, just let it through - maybe it's from a CRD w/ no pod template spec
	hash := k8s.PodTemplateSpecHash(pod.PodTemplateSpecHash)
	if hash == "" {
		return true
	}

	return ContainsHash(filter, hash)
}

// Filter out any pods that are being deleted.
// Filter pods from old replica sets.
// Only keep pods that belong in the current filter.
func FilterPods(filter *KubernetesApplyFilter, pods []v1alpha1.Pod) []v1alpha1.Pod {
	result := []v1alpha1.Pod{}

	// We want to make sure that if one Deployment
	// creates 2 ReplicaSets, we prune pods from the older ReplicaSet.
	newestOwnerByAncestorUID := make(map[string]*v1alpha1.PodOwner)
	for _, pod := range pods {
		if pod.AncestorUID == "" || !hasValidOwner(pod) {
			continue
		}

		owner := pod.Owner
		existing := newestOwnerByAncestorUID[pod.AncestorUID]
		if existing == nil || owner.CreationTimestamp.After(existing.CreationTimestamp.Time) {
			newestOwnerByAncestorUID[pod.AncestorUID] = owner
		}
	}

	for _, pod := range pods {
		// Ignore pods that are currently being deleted.
		if pod.Deleting {
			continue
		}

		// Ignore pods from an old replicaset.
		newestOwner := newestOwnerByAncestorUID[pod.AncestorUID]
		if hasValidOwner(pod) && newestOwner != nil && pod.Owner.Name != newestOwner.Name {
			continue
		}

		// Ignore pods that have a stale pod template hash
		if filter != nil && !HasOKPodTemplateSpecHash(&pod, filter) {
			continue
		}

		// Ignore pods that were tracked by UID but
		// aren't owned by a current Apply.
		if filter != nil && pod.AncestorUID != "" && !ContainsUID(filter, types.UID(pod.AncestorUID)) {
			continue
		}

		result = append(result, pod)
	}

	return result
}

func hasValidOwner(pod v1alpha1.Pod) bool {
	return pod.Owner != nil && pod.Owner.Name != "" && !pod.Owner.CreationTimestamp.IsZero()
}

func MostRecentPod(pod []v1alpha1.Pod) v1alpha1.Pod {
	bestPod := v1alpha1.Pod{}
	found := false

	for _, v := range pod {
		if !found || PodCompare(v, bestPod) {
			bestPod = v
			found = true
		}
	}

	return bestPod
}

// PodCompare is a stable sort order for pods.
func PodCompare(p1 v1alpha1.Pod, p2 v1alpha1.Pod) bool {
	if p1.CreatedAt.After(p2.CreatedAt.Time) {
		return true
	} else if p2.CreatedAt.After(p1.CreatedAt.Time) {
		return false
	}
	return p1.Name > p2.Name
}
