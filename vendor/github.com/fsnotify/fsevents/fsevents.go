//go:build darwin

// Package fsevents provides file system notifications on macOS.
package fsevents

import (
	"sync"
	"syscall"
	"time"
)

// Event represents a single file system notification.
type Event struct {
	// Path holds the path to the item that's changed, relative
	// to its device's root.
	// Use DeviceForPath to determine the absolute path that's
	// being referred to.
	Path string

	// Flags holds details what has happened.
	Flags EventFlags

	// ID holds the event ID.
	//
	// Each event ID comes from the most recent event being reported
	// in the corresponding directory named in the EventStream.Paths field
	// Event IDs all come from a single global source.
	// They are guaranteed to always be increasing, usually in leaps
	// and bounds, even across system reboots and moving drives from
	// one machine to another. If you were to
	// stop processing events from this stream after this event
	// and resume processing them later from a newly-created
	// EventStream, this is the value you would pass for the
	// EventStream.EventID along with Resume=true.
	ID uint64
}

// DeviceForPath returns the device ID for the specified volume.
func DeviceForPath(path string) (int32, error) {
	stat := syscall.Stat_t{}
	if err := syscall.Lstat(path, &stat); err != nil {
		return 0, err
	}
	return stat.Dev, nil
}

// EventStream is the primary interface to FSEvents
// You can provide your own event channel if you wish (or one will be
// created on Start).
//
//	es := &EventStream{Paths: []string{"/tmp"}, Flags: 0}
//	es.Start()
//	es.Stop()
//	...
type EventStream struct {
	stream       fsEventStreamRef
	qref         fsDispatchQueueRef
	hasFinalizer bool
	registryID   uintptr
	uuid         string

	// Events holds the channel on which events will be sent.
	// It's initialized by EventStream.Start if nil.
	Events chan []Event

	// Paths holds the set of paths to watch, each
	// specifying the root of a filesystem hierarchy to be
	// watched for modifications.
	Paths []string

	// Flags specifies what events to receive on the stream.
	Flags CreateFlags

	// Resume specifies that watching should resume from the event
	// specified by EventID.
	Resume bool

	// EventID holds the most recent event ID.
	//
	// NOTE: this is updated asynchronously by the
	// watcher and should not be accessed while
	// the stream has been started.
	EventID uint64

	// Latency holds the number of seconds the service should wait after hearing
	// about an event from the kernel before passing it along to the
	// client via its callback. Specifying a larger value may result
	// in more effective temporal coalescing, resulting in fewer
	// callbacks and greater overall efficiency.
	Latency time.Duration

	// When Device is non-zero, the watcher will watch events on the
	// device with this ID, and the paths in the Paths field are
	// interpreted relative to the device's root.
	//
	// The device ID is the same as the st_dev field from a stat
	// structure of a file on that device or the f_fsid[0] field of
	// a statfs structure.
	Device int32
}

// eventStreamRegistry is a lookup table for EventStream references passed to
// cgo. In Go 1.6+ passing a Go pointer to a Go pointer to cgo is not allowed.
// To get around this issue, we pass only an integer.
type eventStreamRegistry struct {
	sync.Mutex
	m      map[uintptr]*EventStream
	lastID uintptr
}

var registry = eventStreamRegistry{m: map[uintptr]*EventStream{}}

func (r *eventStreamRegistry) Add(e *EventStream) uintptr {
	r.Lock()
	defer r.Unlock()

	r.lastID++
	r.m[r.lastID] = e
	return r.lastID
}

func (r *eventStreamRegistry) Get(i uintptr) *EventStream {
	r.Lock()
	defer r.Unlock()

	return r.m[i]
}

func (r *eventStreamRegistry) Delete(i uintptr) {
	r.Lock()
	defer r.Unlock()

	delete(r.m, i)
}

// Start listening to an event stream. This creates es.Events if it's not already
// a valid channel.
func (es *EventStream) Start() error {
	if es.Events == nil {
		es.Events = make(chan []Event)
	}

	// register eventstream in the local registry for later lookup
	// in C callback
	cbInfo := registry.Add(es)
	es.registryID = cbInfo
	es.uuid = GetDeviceUUID(es.Device)
	err := es.start(es.Paths, cbInfo)
	if err != nil {
		// Remove eventstream from the registry
		registry.Delete(es.registryID)
		es.registryID = 0
	}
	return err
}

// Flush flushes events that have occurred but haven't been delivered.
// If sync is true, it will block until all the events have been delivered,
// otherwise it will return immediately.
func (es *EventStream) Flush(sync bool) {
	flush(es.stream, sync)
}

// Stop stops listening to the event stream.
func (es *EventStream) Stop() {
	if es.stream != nil {
		stop(es.stream, es.qref)
		es.stream = nil
		es.qref = nil
	}

	// Remove eventstream from the registry
	registry.Delete(es.registryID)
	es.registryID = 0
}

// Restart restarts the event listener. This
// can be used to change the current watch flags.
func (es *EventStream) Restart() error {
	es.Stop()
	es.Resume = true
	return es.Start()
}
