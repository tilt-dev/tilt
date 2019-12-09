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

package trace

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/otel/api/core"
)

type Provider interface {
	// Tracer creates a named tracer that implements Tracer interface.
	// If the name is an empty string then provider uses default name.
	Tracer(name string) Tracer
}

type Tracer interface {
	// Start a span.
	Start(ctx context.Context, spanName string, startOpts ...SpanOption) (context.Context, Span)

	// WithSpan wraps the execution of the fn function with a span.
	// It starts a new span, sets it as an active span in the context,
	// executes the fn function and closes the span before returning the result of fn.
	WithSpan(
		ctx context.Context,
		spanName string,
		fn func(ctx context.Context) error,
	) error
}

type EndOptions struct {
	EndTime time.Time
}

type EndOption func(*EndOptions)

func WithEndTime(endTime time.Time) EndOption {
	return func(opts *EndOptions) {
		opts.EndTime = endTime
	}
}

type Span interface {
	// Tracer returns tracer used to create this span. Tracer cannot be nil.
	Tracer() Tracer

	// End completes the span. No updates are allowed to span after it
	// ends. The only exception is setting status of the span.
	End(options ...EndOption)

	// AddEvent adds an event to the span.
	AddEvent(ctx context.Context, msg string, attrs ...core.KeyValue)
	// AddEventWithTimestamp adds an event with a custom timestamp
	// to the span.
	AddEventWithTimestamp(ctx context.Context, timestamp time.Time, msg string, attrs ...core.KeyValue)

	// IsRecording returns true if the span is active and recording events is enabled.
	IsRecording() bool

	// SpanContext returns span context of the span. Returned SpanContext is usable
	// even after the span ends.
	SpanContext() core.SpanContext

	// SetStatus sets the status of the span. The status of the span can be updated
	// even after span ends.
	SetStatus(codes.Code)

	// SetName sets the name of the span.
	SetName(name string)

	// Set span attributes
	SetAttributes(...core.KeyValue)
}

// SpanOption apply changes to SpanOptions.
type SpanOption func(*SpanOptions)

// SpanOptions provides options to set properties of span at the time of starting
// a new span.
type SpanOptions struct {
	Attributes []core.KeyValue
	StartTime  time.Time
	Links      []Link
	Relation   Relation
	Record     bool
	SpanKind   SpanKind
}

// Relation is used to establish relationship between newly created span and the
// other span. The other span could be related as a parent or linked or any other
// future relationship type.
type Relation struct {
	core.SpanContext
	RelationshipType
}

type RelationshipType int

const (
	ChildOfRelationship RelationshipType = iota
	FollowsFromRelationship
)

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
	core.SpanContext
	Attributes []core.KeyValue
}

// SpanKind represents the role of a Span inside a Trace. Often, this defines how a Span
// will be processed and visualized by various backends.
type SpanKind int

const (
	// As a convenience, these match the proto definition, see
	// opentelemetry/proto/trace/v1/trace.proto
	//
	// The unspecified value is not a valid `SpanKind`.  Use
	// `ValidateSpanKind()` to coerce a span kind to a valid
	// value.
	SpanKindUnspecified SpanKind = 0
	SpanKindInternal    SpanKind = 1
	SpanKindServer      SpanKind = 2
	SpanKindClient      SpanKind = 3
	SpanKindProducer    SpanKind = 4
	SpanKindConsumer    SpanKind = 5
)

// ValidateSpanKind returns a valid span kind value.  This will coerce
// invalid values into the default value, SpanKindInternal.
func ValidateSpanKind(spanKind SpanKind) SpanKind {
	switch spanKind {
	case SpanKindInternal,
		SpanKindServer,
		SpanKindClient,
		SpanKindProducer,
		SpanKindConsumer:
		// valid
		return spanKind
	default:
		return SpanKindInternal
	}
}

// String returns the specified name of the SpanKind in lower-case.
func (sk SpanKind) String() string {
	switch sk {
	case SpanKindInternal:
		return "internal"
	case SpanKindServer:
		return "server"
	case SpanKindClient:
		return "client"
	case SpanKindProducer:
		return "producer"
	case SpanKindConsumer:
		return "consumer"
	default:
		return "unspecified"
	}
}

// WithStartTime sets the start time of the span to provided time t, when it is started.
// In absence of this option, wall clock time is used as start time.
// This option is typically used when starting of the span is delayed.
func WithStartTime(t time.Time) SpanOption {
	return func(o *SpanOptions) {
		o.StartTime = t
	}
}

// WithAttributes sets attributes to span. These attributes provides additional
// data about the span.
// Multiple `WithAttributes` options appends the attributes preserving the order.
func WithAttributes(attrs ...core.KeyValue) SpanOption {
	return func(o *SpanOptions) {
		o.Attributes = append(o.Attributes, attrs...)
	}
}

// WithRecord specifies that the span should be recorded.
// Note that the implementation may still override this preference,
// e.g., if the span is a child in an unsampled trace.
func WithRecord() SpanOption {
	return func(o *SpanOptions) {
		o.Record = true
	}
}

// ChildOf. TODO: do we need this?.
func ChildOf(sc core.SpanContext) SpanOption {
	return func(o *SpanOptions) {
		o.Relation = Relation{
			SpanContext:      sc,
			RelationshipType: ChildOfRelationship,
		}
	}
}

// FollowsFrom. TODO: do we need this?.
func FollowsFrom(sc core.SpanContext) SpanOption {
	return func(o *SpanOptions) {
		o.Relation = Relation{
			SpanContext:      sc,
			RelationshipType: FollowsFromRelationship,
		}
	}
}

// LinkedTo allows instantiating a Span with initial Links.
func LinkedTo(sc core.SpanContext, attrs ...core.KeyValue) SpanOption {
	return func(o *SpanOptions) {
		o.Links = append(o.Links, Link{sc, attrs})
	}
}

// WithSpanKind specifies the role a Span on a Trace.
func WithSpanKind(sk SpanKind) SpanOption {
	return func(o *SpanOptions) {
		o.SpanKind = sk
	}
}
