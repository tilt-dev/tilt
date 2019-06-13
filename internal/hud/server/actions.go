package server

import "github.com/windmilleng/tilt/internal/model"

type AppendToTriggerQueueAction struct {
	Name model.ManifestName
}

func (AppendToTriggerQueueAction) Action() {}
