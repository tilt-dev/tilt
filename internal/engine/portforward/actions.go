package portforward

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
)

type PortForwardUpsertAction struct {
	PortForward *PortForward
}

func NewPortForwardUpsertAction(pf *PortForward) PortForwardUpsertAction {
	return PortForwardUpsertAction{PortForward: pf.DeepCopy()}
}

var _ store.Summarizer = PortForwardUpsertAction{}

func (PortForwardUpsertAction) Action() {}

func (a PortForwardUpsertAction) Summarize(s *store.ChangeSummary) {
	s.PortForwards.Add(types.NamespacedName{Name: a.PortForward.Name})
}

type PortForwardDeleteAction struct {
	Name string
}

func NewPortForwardDeleteAction(pfName string) PortForwardDeleteAction {
	return PortForwardDeleteAction{Name: pfName}
}

func (PortForwardDeleteAction) Action() {}

func (a PortForwardDeleteAction) Summarize(s *store.ChangeSummary) {
	s.PodLogStreams.Add(types.NamespacedName{Name: a.Name})
}
