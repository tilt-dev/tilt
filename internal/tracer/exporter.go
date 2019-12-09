package tracer

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/windmilleng/wmclient/pkg/dirs"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Locker sync.Locker

type exporter struct {
	outgoingMu sync.Mutex
	dir        *dirs.WindmillDir

	// members for communicating with the loop() goroutine
	spanDataCh chan *exporttrace.SpanData
	stopCh     chan struct{}

	// members only accessed by the loop() goroutine
	queue      []*exporttrace.SpanData
	needsFlush bool
}

func newExporter(dir *dirs.WindmillDir) (*exporter, error) {
	r := &exporter{
		dir:        dir,
		spanDataCh: make(chan *exporttrace.SpanData),
		stopCh:     make(chan struct{}),
	}
	go r.loop()
	return r, nil
}

const OutgoingFilename = "usage/outgoing.json"

func (e *exporter) loop() {
	var flushingCh chan struct{}
	var timerCh <-chan time.Time
	for {
		select {
		case sd := <-e.spanDataCh:
			e.queue = append(e.queue, sd)
			if timerCh == nil {
				timerCh = time.NewTimer(5 * time.Second).C
			}
		case <-e.stopCh:
			e.stopCh = nil
			e.needsFlush = true
		case <-timerCh:
			timerCh = nil
			e.needsFlush = true
		case <-flushingCh:
			flushingCh = nil
		}

		if !e.needsFlush || flushingCh != nil {
			continue
		}

		if len(e.queue) == 0 {
			e.needsFlush = false
			continue
		}

		flushingCh = make(chan struct{})
		go e.flush(flushingCh)
	}
}

const maxFileSize = 32 * 1024 // 128 MiB

func (e *exporter) flush(flushingCh chan struct{}) {
	// TODO(dbentley): what to do with errors?
	// I think we should have a built-in resource like "tilt_system" that can show errors like this
	e.outgoingMu.Lock()
	defer e.outgoingMu.Unlock()
	defer close(flushingCh)
	q := e.queue
	e.queue = nil

	s, _ := e.dir.ReadFile(OutgoingFilename)

	var existing []*exporttrace.SpanData
	_ = json.Unmarshal([]byte(s), &existing)

	q = append(existing, q...)

	bs, _ := json.MarshalIndent(q, "", "  ")
	for len(bs) > maxFileSize {
		q = q[len(q)/2:]
		bs, _ = json.MarshalIndent(q, "", "  ")
	}

	err := e.dir.WriteFile(OutgoingFilename, string(bs))
	if err != nil {
		// TODO(dmiller)
	}
}

func (e *exporter) OnStart(sd *exporttrace.SpanData) {
}

func (e *exporter) OnEnd(sd *exporttrace.SpanData) {
	e.spanDataCh <- sd
}

func (e *exporter) Shutdown() {
	close(e.stopCh)
}

var _ sdktrace.SpanProcessor = (*exporter)(nil)
