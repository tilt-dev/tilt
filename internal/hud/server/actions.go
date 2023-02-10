package server

import (
	"github.com/tilt-dev/tilt/pkg/model"
)

// TODO: a way to clear an override
type OverrideTriggerModeAction struct {
	ManifestNames []model.ManifestName
	TriggerMode   model.TriggerMode
}

func (OverrideTriggerModeAction) Action() {}
