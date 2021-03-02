package local

import (
	"github.com/tilt-dev/tilt/pkg/model"
)

type CmdCreateAction struct {
	ManifestName model.ManifestName
	Cmd          *Cmd
}

func (CmdCreateAction) Action() {}
