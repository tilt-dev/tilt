package portforwards

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type PortForwardUpsertAction struct {
	PortForward *v1alpha1.PortForward
}

func NewPortForwardUpsertAction(obj *v1alpha1.PortForward) PortForwardUpsertAction {
	return PortForwardUpsertAction{PortForward: obj}
}

func (PortForwardUpsertAction) Action() {}

type PortForwardDeleteAction struct {
	Name string
}

func NewPortForwardDeleteAction(n string) PortForwardDeleteAction {
	return PortForwardDeleteAction{Name: n}
}

func (PortForwardDeleteAction) Action() {}
