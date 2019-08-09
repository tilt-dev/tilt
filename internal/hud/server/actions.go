package server

import "github.com/windmilleng/tilt/pkg/model"

type AppendToTriggerQueueAction struct {
	Name model.ManifestName
}

func (AppendToTriggerQueueAction) Action() {}
