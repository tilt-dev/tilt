package portforward

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
)

type PortForwardCreateAction struct {
	PortForward *PortForward
}

func NewPortForwardCreateAction(pf *PortForward) PortForwardCreateAction {
	return PortForwardCreateAction{PortForward: pf.DeepCopy()}
}

var _ store.Summarizer = PortForwardCreateAction{}

func (PortForwardCreateAction) Action() {}

func (a PortForwardCreateAction) Summarize(s *store.ChangeSummary) {
	s.PortForwards.Add(types.NamespacedName{Name: a.PortForward.Name})
}

type PortForwardDeleteAction struct {
	Name string
}

func (PortForwardDeleteAction) Action() {}

func (a PortForwardDeleteAction) Summarize(s *store.ChangeSummary) {
	s.PodLogStreams.Add(types.NamespacedName{Name: a.Name})
}
