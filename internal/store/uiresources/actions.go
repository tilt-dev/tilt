package uiresources

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type UIResourceUpsertAction struct {
	UIResource *v1alpha1.UIResource
}

func NewUIResourceUpsertAction(obj *v1alpha1.UIResource) UIResourceUpsertAction {
	return UIResourceUpsertAction{UIResource: obj}
}

func (UIResourceUpsertAction) Action() {}

type UIResourceDeleteAction struct {
	Name string
}

func NewUIResourceDeleteAction(n string) UIResourceDeleteAction {
	return UIResourceDeleteAction{Name: n}
}

func (UIResourceDeleteAction) Action() {}
