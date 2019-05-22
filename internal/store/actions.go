package store

import (
	"time"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

type ErrorAction struct {
	Error error
}

func (ErrorAction) Action() {}

func NewErrorAction(err error) ErrorAction {
	return ErrorAction{Error: err}
}

type LogAction struct {
	LogEvent
}

func (LogAction) Action() {}

type LogEvent struct {
	Timestamp time.Time
	Msg       []byte
}

func (le LogEvent) Time() time.Time {
	return le.Timestamp
}

func (le LogEvent) Message() []byte {
	return le.Msg
}

func NewLogEvent(b []byte) LogEvent {
	return LogEvent{
		Timestamp: time.Now(),
		Msg:       b,
	}
}

type ResetRestartsAction struct {
	ManifestName model.ManifestName
}

func (ResetRestartsAction) Action() {}

func NewResetRestartsAction(name model.ManifestName) ResetRestartsAction {
	return ResetRestartsAction{
		ManifestName: name,
	}
}

type AnalyticsOptAction struct {
	Opt analytics.Opt
}

func (AnalyticsOptAction) Action() {}

// Indicates nudge surfaced for the first time
type AnalyticsNudgeSurfacedAction struct {
	Opt analytics.Opt
}

func (AnalyticsNudgeSurfacedAction) Action() {}
