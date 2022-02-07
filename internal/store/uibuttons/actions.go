package uibuttons

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type UIButtonUpsertAction struct {
	UIButton *v1alpha1.UIButton
}

var _ store.Summarizer = UIButtonUpsertAction{}

func NewUIButtonUpsertAction(obj *v1alpha1.UIButton) UIButtonUpsertAction {
	return UIButtonUpsertAction{UIButton: obj}
}

func (a UIButtonUpsertAction) Summarize(summary *store.ChangeSummary) {
	summary.UIButtons.Add(types.NamespacedName{Name: a.UIButton.Name})
}

func (UIButtonUpsertAction) Action() {}

type UIButtonDeleteAction struct {
	Name string
}

var _ store.Summarizer = UIButtonDeleteAction{}

func NewUIButtonDeleteAction(n string) UIButtonDeleteAction {
	return UIButtonDeleteAction{Name: n}
}

func (a UIButtonDeleteAction) Summarize(summary *store.ChangeSummary) {
	summary.UIButtons.Add(types.NamespacedName{Name: a.Name})
}

func (UIButtonDeleteAction) Action() {}
