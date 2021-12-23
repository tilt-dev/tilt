package uiresources

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type UIResourceUpsertAction struct {
	UIResource *v1alpha1.UIResource
}

var _ store.Summarizer = UIResourceUpsertAction{}

func NewUIResourceUpsertAction(obj *v1alpha1.UIResource) UIResourceUpsertAction {
	return UIResourceUpsertAction{UIResource: obj}
}

func (a UIResourceUpsertAction) Summarize(summary *store.ChangeSummary) {
	summary.UIResources.Add(types.NamespacedName{Name: a.UIResource.Name})
}

func (UIResourceUpsertAction) Action() {}

type UIResourceDeleteAction struct {
	Name string
}

var _ store.Summarizer = UIResourceDeleteAction{}

func NewUIResourceDeleteAction(n string) UIResourceDeleteAction {
	return UIResourceDeleteAction{Name: n}
}

func (a UIResourceDeleteAction) Summarize(summary *store.ChangeSummary) {
	summary.UIResources.Add(types.NamespacedName{Name: a.Name})
}

func (UIResourceDeleteAction) Action() {}
