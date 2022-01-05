package clusters

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type ClusterUpsertAction struct {
	Cluster *v1alpha1.Cluster
}

func NewClusterUpsertAction(obj *v1alpha1.Cluster) ClusterUpsertAction {
	return ClusterUpsertAction{Cluster: obj}
}

func (a ClusterUpsertAction) Summarize(summary *store.ChangeSummary) {
	summary.Clusters.Add(types.NamespacedName{
		Namespace: a.Cluster.Namespace,
		Name:      a.Cluster.Name,
	})
}

func (ClusterUpsertAction) Action() {}

type ClusterDeleteAction struct {
	Name string
}

func NewClusterDeleteAction(n string) ClusterDeleteAction {
	return ClusterDeleteAction{Name: n}
}

func (ClusterDeleteAction) Action() {}

func (a ClusterDeleteAction) Summarize(summary *store.ChangeSummary) {
	summary.Clusters.Add(types.NamespacedName{
		Name: a.Name,
	})
}
