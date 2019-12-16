package tracer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Exporter does 3 things:
// 1) Accepts spans from OpenTelemetry.
// 2) Stores spans (for now, in memory)
// 3) Allows consumers to read spans they might want to send elsewhere
// Numbers 2 and 3 access the same data, and so it's a concurrency issue.
type Exporter struct {

	// members for communicating with the loop() goroutine

	// for OpenTelemetry Exporter
	spanDataCh chan *exporttrace.SpanData

	// for SpanSource
	readReqCh  chan struct{}
	readRespCh chan readResp
}

// SpanSource is the interface for consumers (generally telemetry.Controller)
type SpanSource interface {
	// GetOutgoingSpans gives a consumer access to spans they should send
	// Data may be nil (so the client knows there won't be any data)
	// The client must send a value over doneCh; True indicates the
	// SpanSource should remove the data read; false indicates SpanSource should retain the data.
	GetOutgoingSpans() (data io.Reader, doneCh chan<- bool, err error)
}

func NewExporter(ctx context.Context) *Exporter {
	r := &Exporter{
		spanDataCh: make(chan *exporttrace.SpanData),
		readReqCh:  make(chan struct{}),
		readRespCh: make(chan readResp),
	}
	go r.loop(ctx)
	return r
}

func (e *Exporter) loop(ctx context.Context) {
	// spans that have come in and are waiting to be read by a consumer
	var queue []*exporttrace.SpanData

	// what a consumer (generally telemetry.Controller) is reading right now
	// we keep track of it so that if there's a script error, the data isn't lost forever
	var beingRead []*exporttrace.SpanData
	var readDoneCh chan bool

	for {
		select {
		// New work coming in
		case sd := <-e.spanDataCh:
			// add to the queue
			queue = appendAndTrim(queue, sd)
		case <-e.readReqCh:
			// send the queue to the reader
			if len(queue) == 0 {
				// There's nothing pending, so send a nil reader and a channel the client can write to (because it's buffered with length 1)
				e.readRespCh <- readResp{doneCh: make(chan bool, 1)}
				break
			}
			r, err := e.makeReader(queue)
			if err != nil {
				// oh wait, there's a problem encoding?
				// tell the reader, and then clear the queue
				e.readRespCh <- readResp{err: err}
				queue = nil
				break
			}

			// there's now a read in flight
			readDoneCh = make(chan bool)
			beingRead, queue = queue, nil
			e.readRespCh <- readResp{r: r, doneCh: readDoneCh}

		// In-flight operations finishing
		case delete := <-readDoneCh:
			if !delete {
				// They don't want to consume these yet, so add them back to the front of the queue
				queue = appendAndTrim(beingRead, queue...)
			}
			beingRead = nil
		}
	}
}

// OpenTelemetry exporter methods
func (e *Exporter) OnStart(sd *exporttrace.SpanData) {
}

func (e *Exporter) OnEnd(sd *exporttrace.SpanData) {
	e.spanDataCh <- sd
}

func (e *Exporter) Shutdown() {
	// TODO(dbentley): handle shutdown
}

// SpanSource
func (e *Exporter) GetOutgoingSpans() (io.Reader, chan<- bool, error) {
	e.readReqCh <- struct{}{}
	resp := <-e.readRespCh
	return resp.r, resp.doneCh, resp.err
}

type readResp struct {
	r      io.Reader
	doneCh chan bool
	err    error
}

func (e *Exporter) makeReader(spans []*exporttrace.SpanData) (io.Reader, error) {
	var b strings.Builder
	w := json.NewEncoder(&b)
	for _, span := range spans {
		if err := w.Encode(span); err != nil {
			return nil, fmt.Errorf("Error marshaling %v: %v", span, err)
		}
	}

	return strings.NewReader(b.String()), nil
}

const maxQueueSize = 1024 // round number that can hold a fair bit of data

func appendAndTrim(lst1 []*exporttrace.SpanData, lst2 ...*exporttrace.SpanData) []*exporttrace.SpanData {
	r := append(lst1, lst2...)
	if len(r) <= maxQueueSize {
		return r
	}
	elemsToRemove := len(r) - maxQueueSize
	return r[elemsToRemove:]
}

var _ sdktrace.SpanProcessor = (*Exporter)(nil)
var _ SpanSource = (*Exporter)(nil)
