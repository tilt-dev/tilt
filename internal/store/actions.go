package store

import (
	"time"
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
