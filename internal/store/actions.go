package store

import (
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/wmclient/pkg/analytics"
	v1 "k8s.io/api/core/v1"
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

type K8SEventAction struct {
	Event *v1.Event
}

func (K8SEventAction) Action() {}

func NewK8SEventAction(event *v1.Event) K8SEventAction {
	return K8SEventAction{event}
}

func (kEvt K8SEventAction) Time() time.Time {
	return kEvt.Event.LastTimestamp.Time
}

func (kEvt K8SEventAction) Message() []byte {
	return []byte(kEvt.MessageRaw() + "\n")
}

func (kEvt K8SEventAction) MessageRaw() string {
	// TODO(maia): obj reference, namespace, etc.
	return fmt.Sprintf("[K8S EVENT] %s", kEvt.Event.Message)
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

type AnalyticsNudgeSurfacedAction struct{}

func (AnalyticsNudgeSurfacedAction) Action() {}
