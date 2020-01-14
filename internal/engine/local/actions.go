package local

import (
	"github.com/windmilleng/tilt/pkg/model"
)

type LocalServeStatusAction struct {
	ManifestName model.ManifestName
	Status       model.RuntimeStatus
	PID          int // 0 if there's no process running
	SpanID       model.LogSpanID
}

func (LocalServeStatusAction) Action() {}
