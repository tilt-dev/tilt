package tracer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/tilt-dev/tilt/internal/tracer/exptel"
)

// SpanCollector does 3 things:
// 1) Accepts spans from OpenTelemetry.
// 2) Stores spans (for now, in memory)
// 3) Allows consumers to read spans they might want to send elsewhere
// Numbers 2 and 3 access the same data, and so it's a concurrency issue.
type SpanCollector struct {
	// members for communicating with the loop() goroutine

	// for OpenTelemetry SpanCollector
	spanDataCh chan trace.ReadOnlySpan

	// for SpanSource
	readReqCh chan chan []trace.ReadOnlySpan
	requeueCh chan []trace.ReadOnlySpan
}

// SpanSource is the interface for consumers (generally telemetry.Controller)
type SpanSource interface {
	// GetOutgoingSpans gives a consumer access to spans they should send
	// If there are no outgoing spans, err will be io.EOF
	// rejectFn allows client to reject spans, so they can be requeued
	// rejectFn must be called, if at all, before the next call to GetOutgoingSpans
	GetOutgoingSpans() (data io.Reader, rejectFn func(), err error)

	// Close closes the SpanSource; the client may not interact with this SpanSource after calling Close
	Close() error
}

func NewSpanCollector(ctx context.Context) *SpanCollector {
	r := &SpanCollector{
		spanDataCh: make(chan trace.ReadOnlySpan),
		readReqCh:  make(chan chan []trace.ReadOnlySpan),
		requeueCh:  make(chan []trace.ReadOnlySpan),
	}
	go r.loop(ctx)
	return r
}

func (c *SpanCollector) loop(ctx context.Context) {
	// spans that have come in and are waiting to be read by a consumer
	var queue []trace.ReadOnlySpan

	for {
		if c.spanDataCh == nil && c.readReqCh == nil {
			return
		}
		select {
		// New work coming in
		case sd, ok := <-c.spanDataCh:
			if !ok {
				c.spanDataCh = nil
				break
			}
			// add to the queue
			queue = appendAndTrim(queue, sd)
		case respCh, ok := <-c.readReqCh:
			if !ok {
				c.readReqCh = nil
				break
			}
			// send the queue to the reader
			respCh <- queue
			queue = nil
		// In-flight operations finishing
		case sds := <-c.requeueCh:
			queue = appendAndTrim(sds, queue...)
		}
	}
}

// OpenTelemetry exporter methods

func (c *SpanCollector) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	for _, s := range spans {
		select {
		case c.spanDataCh <- s:
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

func (c *SpanCollector) Shutdown(ctx context.Context) error {
	close(c.spanDataCh)
	return nil
}

// SpanSource
func (c *SpanCollector) GetOutgoingSpans() (io.Reader, func(), error) {
	readCh := make(chan []trace.ReadOnlySpan)
	c.readReqCh <- readCh
	spans := <-readCh

	if len(spans) == 0 {
		return nil, nil, io.EOF
	}

	var b strings.Builder
	w := json.NewEncoder(&b)
	for i := range spans {
		span := exptel.NewSpanFromOtel(spans[i], tracerName+"/")
		if err := w.Encode(span); err != nil {
			return nil, nil, fmt.Errorf("Error marshaling %v: %v", span, err)
		}
	}

	rejectFn := func() {
		c.requeueCh <- spans
	}

	return strings.NewReader(b.String()), rejectFn, nil
}

func (c *SpanCollector) Close() error {
	close(c.readReqCh)
	return nil
}

const maxQueueSize = 1024 // round number that can hold a fair bit of data

func appendAndTrim(lst1 []trace.ReadOnlySpan, lst2 ...trace.ReadOnlySpan) []trace.ReadOnlySpan {
	r := append(lst1, lst2...)
	if len(r) <= maxQueueSize {
		return r
	}
	elemsToRemove := len(r) - maxQueueSize
	return r[elemsToRemove:]
}

var _ trace.SpanExporter = (*SpanCollector)(nil)
var _ SpanSource = (*SpanCollector)(nil)
