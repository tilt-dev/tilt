package logstore

import (
	"time"

	"github.com/windmilleng/tilt/pkg/model"
)

type testLogEvent struct {
	name    model.ManifestName
	ts      time.Time
	message string
}

func (l testLogEvent) Message() []byte {
	return []byte(l.message)
}

func (l testLogEvent) Time() time.Time {
	return l.ts
}

func (l testLogEvent) Source() model.ManifestName {
	return l.name
}

func newGlobalTestLogEvent(message string) testLogEvent {
	return newTestLogEvent("", time.Now(), message)
}

func newTestLogEvent(name model.ManifestName, ts time.Time, message string) testLogEvent {
	return testLogEvent{
		name:    name,
		ts:      ts,
		message: message,
	}
}
