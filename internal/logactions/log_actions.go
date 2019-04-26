package logactions

import (
	"time"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

type LogEvent struct {
	ts      time.Time
	message []byte
}

func (le LogEvent) Time() time.Time {
	return le.ts
}

func (le LogEvent) Message() []byte {
	return le.message
}

func NewLogEvent(b []byte) LogEvent {
	return LogEvent{
		ts:      time.Now(),
		message: b,
	}
}

type LogAction struct {
	LogEvent
}

func (LogAction) Action() {}

type BuildLogAction struct {
	LogEvent
	ManifestName model.ManifestName
}

func (BuildLogAction) Action() {}

type PodLogAction struct {
	LogEvent
	ManifestName model.ManifestName
	PodID        k8s.PodID
}

func (PodLogAction) Action() {}

type DockerComposeEventAction struct {
	Event dockercompose.Event
}

func (DockerComposeEventAction) Action() {}

type DockerComposeLogAction struct {
	LogEvent
	ManifestName model.ManifestName
}

func (DockerComposeLogAction) Action() {}

type TiltfileLogAction struct {
	LogEvent
}

func (TiltfileLogAction) Action() {}
