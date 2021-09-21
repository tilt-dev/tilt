package exptel

import (
	"encoding/hex"
	"encoding/json"

	"go.opentelemetry.io/otel/trace"
)

// SpanContext contains basic information about the span - its trace
// ID, span ID and trace flags.
type SpanContext struct {
	TraceID    TraceID
	SpanID     SpanID
	TraceFlags byte
}

func NewSpanContextFromOtel(c trace.SpanContext) SpanContext {
	return SpanContext{
		TraceID:    TraceID(c.TraceID()),
		SpanID:     SpanID(c.SpanID()),
		TraceFlags: byte(c.TraceFlags()),
	}
}

// TraceID is a unique identity of a trace.
type TraceID [16]byte

// MarshalJSON implements a custom marshal function to encode TraceID
// as a hex string.
func (t TraceID) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(t[:]))
}

// SpanID is a unique identify of a span in a trace.
type SpanID [8]byte

// MarshalJSON implements a custom marshal function to encode SpanID
// as a hex string.
func (s SpanID) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(s[:]))
}
