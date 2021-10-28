package liveupdates

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type LiveUpdateUpsertAction struct {
	LiveUpdate *v1alpha1.LiveUpdate
}

func NewLiveUpdateUpsertAction(obj *v1alpha1.LiveUpdate) LiveUpdateUpsertAction {
	return LiveUpdateUpsertAction{LiveUpdate: obj}
}

func (LiveUpdateUpsertAction) Action() {}

type LiveUpdateDeleteAction struct {
	Name string
}

func NewLiveUpdateDeleteAction(n string) LiveUpdateDeleteAction {
	return LiveUpdateDeleteAction{Name: n}
}

func (LiveUpdateDeleteAction) Action() {}
