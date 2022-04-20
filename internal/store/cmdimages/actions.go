package cmdimages

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type CmdImageUpsertAction struct {
	CmdImage *v1alpha1.CmdImage
}

func NewCmdImageUpsertAction(obj *v1alpha1.CmdImage) CmdImageUpsertAction {
	return CmdImageUpsertAction{CmdImage: obj}
}

func (CmdImageUpsertAction) Action() {}

type CmdImageDeleteAction struct {
	Name string
}

func NewCmdImageDeleteAction(n string) CmdImageDeleteAction {
	return CmdImageDeleteAction{Name: n}
}

func (CmdImageDeleteAction) Action() {}
