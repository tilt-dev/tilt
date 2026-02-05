package webview

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogLevel represents the severity level of a log entry.
type LogLevel string

const (
	LogLevel_NONE    LogLevel = "NONE"
	LogLevel_INFO    LogLevel = "INFO"
	LogLevel_VERBOSE LogLevel = "VERBOSE"
	LogLevel_DEBUG   LogLevel = "DEBUG"
	LogLevel_WARN    LogLevel = "WARN"
	LogLevel_ERROR   LogLevel = "ERROR"
)

type LogSegment struct {
	SpanId string           `json:"spanId,omitempty"`
	Time   metav1.MicroTime `json:"time,omitempty"`
	Text   string           `json:"text,omitempty"`
	Level  LogLevel         `json:"level,omitempty"`
	// When we store warnings in the LogStore, we break them up into lines and
	// store them as a series of line segments. 'anchor' marks the beginning of a
	// series of logs that should be kept together.
	//
	// Anchor warning1, line1
	//        warning1, line2
	// Anchor warning2, line1
	Anchor bool `json:"anchor,omitempty"`

	// Context-specific optional fields for a log segment.
	// Used for experimenting with new types of log metadata.
	Fields map[string]string `json:"fields,omitempty"`
}

type LogSpan struct {
	ManifestName string `json:"manifestName,omitempty"`
}

type LogList struct {
	Spans    map[string]*LogSpan `json:"spans,omitempty"`
	Segments []*LogSegment       `json:"segments,omitempty"`
	// FromCheckpoint and ToCheckpoint express an interval on the
	// central log-store, with an inclusive start and an exclusive end
	//
	// [FromCheckpoint, ToCheckpoint)
	//
	// An interval of [0, 0) means that the server isn't using
	// the incremental load protocol.
	//
	// An interval of [-1, -1) means that the server doesn't have new logs
	// to send down.
	FromCheckpoint int32 `json:"fromCheckpoint,omitempty"`
	ToCheckpoint   int32 `json:"toCheckpoint,omitempty"`
}
