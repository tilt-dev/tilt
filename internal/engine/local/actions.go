package local

import (
	"github.com/windmilleng/tilt/pkg/model"
)

type LocalServeStatusAction struct {
	ManifestName model.ManifestName
	SequenceNum  int
	Status       model.RuntimeStatus
}

func (LocalServeStatusAction) Action() {}
