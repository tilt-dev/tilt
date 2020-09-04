package metrics

import (
	"sync"

	"go.opencensus.io/stats/view"
)

// Glue code between the Tilt subscriber system and the opencensus metrics system.

func NewDeferredExporter() *DeferredExporter {
	return &DeferredExporter{}
}

type RemoteExporter interface {
	view.Exporter
	Flush()
	Stop() error
}

type DeferredExporter struct {
	mu       sync.Mutex
	remote   RemoteExporter
	deferred []*view.Data
}

func (d *DeferredExporter) Flush() {
	view.Flush()

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.remote == nil {
		return
	}
	d.remote.Flush()
}

func (d *DeferredExporter) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.remote == nil {
		return nil
	}
	return d.remote.Stop()
}

func (d *DeferredExporter) SetRemote(remote RemoteExporter) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	oldRemote := d.remote
	d.remote = remote
	for _, v := range d.deferred {
		d.remote.ExportView(v)
	}
	d.deferred = nil

	if oldRemote == nil {
		return nil
	}

	oldRemote.Flush()
	return oldRemote.Stop()
}

func (d *DeferredExporter) ExportView(viewData *view.Data) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.remote == nil {
		d.deferred = append(d.deferred, viewData)
		return
	}
	d.remote.ExportView(viewData)
}
