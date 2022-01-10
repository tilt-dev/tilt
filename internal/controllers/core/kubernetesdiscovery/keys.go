package kubernetesdiscovery

import (
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// nsKey is a tuple of Cluster metadata and the K8s namespace being watched.
//
// If multiple clusters are are in use, it's possible to watch the same namespace
// in both, so the namespace name alone is not sufficient. Similarly, a Cluster
// object can change, so there might be multiple revisions active at once.
type nsKey struct {
	cluster   clusterKey
	namespace string
}

func newNsKey(cluster *v1alpha1.Cluster, ns string) nsKey {
	return nsKey{
		cluster:   newClusterKey(cluster),
		namespace: ns,
	}
}

// uidKey is a tuple of Cluster metadata and the UID being watched.
//
// If multiple clusters are are in use, it's possible (albeit very unlikely) to
// watch the same UID in both, so the UID alone is not sufficient. Similarly, a
// Cluster object can change, so there might be multiple revisions active at
// once.
type uidKey struct {
	cluster clusterKey
	uid     types.UID
}

func newUIDKey(cluster *v1alpha1.Cluster, uid types.UID) uidKey {
	return uidKey{
		cluster: newClusterKey(cluster),
		uid:     uid,
	}
}

type clusterKey struct {
	name     types.NamespacedName
	revision time.Time
}

func newClusterKey(cluster *v1alpha1.Cluster) clusterKey {
	return clusterKey{
		name:     apis.Key(cluster),
		revision: cluster.Status.ConnectedAt.Time,
	}
}
