package tracer

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/windmilleng/wmclient/pkg/dirs"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/windmilleng/tilt/pkg/logger"
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

func newExporter(ctx context.Context, dir *dirs.WindmillDir) *exporter {
	r := &exporter{
		dir:        dir,
		spanDataCh: make(chan *exporttrace.SpanData),
		stopCh:     make(chan struct{}),
	}
	go r.loop(ctx)
	return r
}

const OutgoingFilename = "usage/outgoing.json"

func (e *exporter) loop(ctx context.Context) {
	var flushingCh chan struct{}
	var timerCh <-chan time.Time
	for {
		select {
		case sd := <-e.spanDataCh:
			e.queue = append(e.queue, sd)
			if timerCh == nil {
				timerCh = time.After(5 * time.Second)
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
		go e.flush(ctx, flushingCh)
	}
}

const maxFileSize = 32 * 1024 // 128 MiB

func (e *exporter) flush(ctx context.Context, flushingCh chan struct{}) {
	e.outgoingMu.Lock()
	defer e.outgoingMu.Unlock()
	defer close(flushingCh)
	q := e.queue
	e.queue = nil

	s, err := e.dir.ReadFile(OutgoingFilename)
	if err != nil {
		logger.Get(ctx).Infof("Error reading %s: %v", OutgoingFilename, err)
		return
	}

	var existing []*exporttrace.SpanData
	err = json.Unmarshal([]byte(s), &existing)
	if err != nil {
		logger.Get(ctx).Infof("Error unmarshaling JSON: %v", err)
		return
	}

	q = append(existing, q...)

	bs, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		logger.Get(ctx).Infof("Error indenting JSON: %v", err)
		return
	}
	for len(bs) > maxFileSize {
		q = q[len(q)/2:]
		bs, _ = json.MarshalIndent(q, "", "  ")
	}

	err = e.dir.WriteFile(OutgoingFilename, string(bs))
	if err != nil {
		logger.Get(ctx).Infof("Error writing %s: %v", OutgoingFilename, err)
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
