package logstore

import (
	"time"

	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type testLogEvent struct {
	name    model.ManifestName
	level   logger.Level
	ts      time.Time
	fields  logger.Fields
	message string
}

func (l testLogEvent) Message() []byte {
	return []byte(l.message)
}

func (l testLogEvent) Level() logger.Level {
	return l.level
}

func (l testLogEvent) Time() time.Time {
	return l.ts
}

func (l testLogEvent) ManifestName() model.ManifestName {
	return l.name
}

func (l testLogEvent) Fields() logger.Fields {
	return l.fields
}

func (l testLogEvent) SpanID() SpanID {
	return SpanID(l.name)
}

func newGlobalTestLogEvent(message string) testLogEvent {
	return newTestLogEvent("", time.Now(), message)
}

func newGlobalLevelTestLogEvent(message string, level logger.Level) testLogEvent {
	event := newTestLogEvent("", time.Now(), message)
	event.level = level
	return event
}

func newTestLogEvent(name model.ManifestName, ts time.Time, message string) testLogEvent {
	return testLogEvent{
		name:    name,
		level:   logger.InfoLvl,
		ts:      ts,
		message: message,
	}
}
