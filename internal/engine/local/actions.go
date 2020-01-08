package local

import (
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

type LocalServeLogAction struct {
	store.LogEvent
}

func (LocalServeLogAction) Action() {}

type LocalServeStatusAction struct {
	ManifestName model.ManifestName
	SequenceNum  int
	Status       Status
}

func (LocalServeStatusAction) Action() {}
