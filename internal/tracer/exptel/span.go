// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exptel

import (
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// SpanKind represents the role of a Span inside a Trace. Often, this defines how a Span
// will be processed and visualized by various backends.
type SpanKind int

// ExperimentalTelemetrySpan contains all the information collected by a span.
type ExperimentalTelemetrySpan struct {
	SpanContext  SpanContext
	ParentSpanID SpanID
	SpanKind     SpanKind
	Name         string
	StartTime    time.Time
	// The wall clock time of EndTime will be adjusted to always be offset
	// from StartTime by the duration of the span.
	EndTime                  time.Time
	Attributes               []KeyValue
	MessageEvents            []Event
	Links                    []Link
	Status                   Code
	HasRemoteParent          bool
	DroppedAttributeCount    int
	DroppedMessageEventCount int
	DroppedLinkCount         int

	// ChildSpanCount holds the number of child span created for this span.
	ChildSpanCount int
}

// NewSpanFromOtel converts an OpenTelemetry span to a type suitable for JSON marshaling for usage with `experimental_telemetry_cmd`.
//
// This format corresponds to the JSON marshaling from opentelemetry-go v0.2.0. See the package docs for more details.
//
// spanNamePrefix can be used to provide a prefix for the outgoing span name to mimic old opentelemetry-go behavior
// which would prepend the tracer name to each span. (See https://github.com/open-telemetry/opentelemetry-go/pull/430).
func NewSpanFromSpanSnapshot(s *sdktrace.SpanSnapshot, spanNamePrefix string) ExperimentalTelemetrySpan {
	name := s.Name
	if spanNamePrefix != "" && name != "" {
		name = spanNamePrefix + name
	}

	return ExperimentalTelemetrySpan{
		SpanContext:              NewSpanContextFromOtel(s.SpanContext),
		ParentSpanID:             SpanID(s.Parent.SpanID()),
		SpanKind:                 SpanKind(s.SpanKind),
		Name:                     name,
		StartTime:                s.StartTime,
		EndTime:                  s.EndTime,
		Attributes:               NewKeyValuesFromOtel(s.Attributes),
		MessageEvents:            NewEventsFromOtel(s.MessageEvents),
		Links:                    NewLinksFromOtel(s.Links),
		Status:                   Code(s.StatusCode),
		HasRemoteParent:          s.Parent.IsRemote(),
		DroppedAttributeCount:    s.DroppedAttributeCount,
		DroppedMessageEventCount: s.DroppedMessageEventCount,
		DroppedLinkCount:         s.DroppedLinkCount,
		ChildSpanCount:           s.ChildSpanCount,
	}
}

// NewSpanFromOtel converts an OpenTelemetry span to a type suitable for JSON marshaling for usage with `experimental_telemetry_cmd`.
//
// This format corresponds to the JSON marshaling from opentelemetry-go v0.2.0. See the package docs for more details.
//
// spanNamePrefix can be used to provide a prefix for the outgoing span name to mimic old opentelemetry-go behavior
// which would prepend the tracer name to each span. (See https://github.com/open-telemetry/opentelemetry-go/pull/430).
func NewSpanFromOtel(sd sdktrace.ReadOnlySpan, spanNamePrefix string) ExperimentalTelemetrySpan {
	name := sd.Name()
	if spanNamePrefix != "" && name != "" {
		name = spanNamePrefix + name
	}

	return ExperimentalTelemetrySpan{
		SpanContext:              NewSpanContextFromOtel(sd.SpanContext()),
		ParentSpanID:             SpanID(sd.Parent().SpanID()),
		SpanKind:                 SpanKind(sd.SpanKind()),
		Name:                     name,
		StartTime:                sd.StartTime(),
		EndTime:                  sd.EndTime(),
		Attributes:               NewKeyValuesFromOtel(sd.Attributes()),
		MessageEvents:            NewEventsFromOtel(sd.Events()),
		Links:                    NewLinksFromOtel(sd.Links()),
		Status:                   Code(sd.StatusCode()),
		HasRemoteParent:          sd.Parent().IsRemote(),
		DroppedAttributeCount:    sd.Snapshot().DroppedAttributeCount,
		DroppedMessageEventCount: sd.Snapshot().DroppedMessageEventCount,
		DroppedLinkCount:         sd.Snapshot().DroppedLinkCount,
		ChildSpanCount:           sd.Snapshot().ChildSpanCount,
	}
}

type Code uint32

// Event is used to describe an Event with a message string and set of
// Attributes.
type Event struct {
	// Message describes the Event.
	Message string

	// Attributes contains a list of keyvalue pairs.
	Attributes []KeyValue

	// Time is the time at which this event was recorded.
	Time time.Time
}

func NewEventsFromOtel(e []trace.Event) []Event {
	if e == nil {
		return nil
	}

	out := make([]Event, len(e))
	for i := range e {
		out[i] = Event{
			Message:    e[i].Name,
			Attributes: NewKeyValuesFromOtel(e[i].Attributes),
			Time:       e[i].Time,
		}
	}
	return out
}

// Link is used to establish relationship between two spans within the same Trace or
// across different Traces. Few examples of Link usage.
//   1. Batch Processing: A batch of elements may contain elements associated with one
//      or more traces/spans. Since there can only be one parent SpanContext, Link is
//      used to keep reference to SpanContext of all elements in the batch.
//   2. Public Endpoint: A SpanContext in incoming client request on a public endpoint
//      is untrusted from service provider perspective. In such case it is advisable to
//      start a new trace with appropriate sampling decision.
//      However, it is desirable to associate incoming SpanContext to new trace initiated
//      on service provider side so two traces (from Client and from Service Provider) can
//      be correlated.
type Link struct {
	SpanContext
	Attributes []KeyValue
}

func NewLinksFromOtel(l []trace.Link) []Link {
	if l == nil {
		return nil
	}

	out := make([]Link, len(l))
	for i := range l {
		out[i] = Link{
			SpanContext: NewSpanContextFromOtel(l[i].SpanContext),
			Attributes:  NewKeyValuesFromOtel(l[i].Attributes),
		}
	}
	return out
}
