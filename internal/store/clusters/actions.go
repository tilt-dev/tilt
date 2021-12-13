package clusters

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type ClusterUpsertAction struct {
	Cluster *v1alpha1.Cluster
}

func NewClusterUpsertAction(obj *v1alpha1.Cluster) ClusterUpsertAction {
	return ClusterUpsertAction{Cluster: obj}
}

func (ClusterUpsertAction) Action() {}

type ClusterDeleteAction struct {
	Name string
}

func NewClusterDeleteAction(n string) ClusterDeleteAction {
	return ClusterDeleteAction{Name: n}
}

func (ClusterDeleteAction) Action() {}
