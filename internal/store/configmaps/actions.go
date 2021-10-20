package configmaps

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type ConfigMapUpsertAction struct {
	ConfigMap *v1alpha1.ConfigMap
}

func NewConfigMapUpsertAction(obj *v1alpha1.ConfigMap) ConfigMapUpsertAction {
	return ConfigMapUpsertAction{ConfigMap: obj}
}

func (ConfigMapUpsertAction) Action() {}

type ConfigMapDeleteAction struct {
	Name string
}

func NewConfigMapDeleteAction(n string) ConfigMapDeleteAction {
	return ConfigMapDeleteAction{Name: n}
}

func (ConfigMapDeleteAction) Action() {}
