//go:build darwin

package fsevents

/*
#cgo LDFLAGS: -framework CoreServices
#include <CoreServices/CoreServices.h>
#include <sys/stat.h>

static CFArrayRef ArrayCreateMutable(int len) {
	return CFArrayCreateMutable(NULL, len, &kCFTypeArrayCallBacks);
}

extern void fsevtCallback(FSEventStreamRef p0, uintptr_t info, size_t p1, char** p2, FSEventStreamEventFlags* p3, FSEventStreamEventId* p4);

static FSEventStreamRef EventStreamCreateRelativeToDevice(FSEventStreamContext * context, uintptr_t info, dev_t dev, CFArrayRef paths, FSEventStreamEventId since, CFTimeInterval latency, FSEventStreamCreateFlags flags) {
	context->info = (void*) info;
	return FSEventStreamCreateRelativeToDevice(NULL, (FSEventStreamCallback) fsevtCallback, context, dev, paths, since, latency, flags);
}

static FSEventStreamRef EventStreamCreate(FSEventStreamContext * context, uintptr_t info, CFArrayRef paths, FSEventStreamEventId since, CFTimeInterval latency, FSEventStreamCreateFlags flags) {
	context->info = (void*) info;
	return FSEventStreamCreate(NULL, (FSEventStreamCallback) fsevtCallback, context, paths, since, latency, flags);
}

static void DispatchQueueRelease(dispatch_queue_t queue) {
	dispatch_release(queue);
}
*/
import "C"
import (
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"runtime"
	"time"
	"unsafe"
)

// CreateFlags specifies what events will be seen in an event stream.
type CreateFlags uint32

const (
	// NoDefer sends events on the leading edge (for interactive applications).
	// By default events are delivered after latency seconds (for background tasks).
	//
	// Affects the meaning of the EventStream.Latency parameter. If you specify
	// this flag and more than latency seconds have elapsed since
	// the last event, your app will receive the event immediately.
	// The delivery of the event resets the latency timer and any
	// further events will be delivered after latency seconds have
	// elapsed. This flag is useful for apps that are interactive
	// and want to react immediately to changes but avoid getting
	// swamped by notifications when changes are occurring in rapid
	// succession. If you do not specify this flag, then when an
	// event occurs after a period of no events, the latency timer
	// is started. Any events that occur during the next latency
	// seconds will be delivered as one group (including that first
	// event). The delivery of the group of events resets the
	// latency timer and any further events will be delivered after
	// latency seconds. This is the default behavior and is more
	// appropriate for background, daemon or batch processing apps.
	NoDefer = CreateFlags(C.kFSEventStreamCreateFlagNoDefer)

	// WatchRoot requests notifications of changes along the path to
	// the path(s) you're watching. For example, with this flag, if
	// you watch "/foo/bar" and it is renamed to "/foo/bar.old", you
	// would receive a RootChanged event. The same is true if the
	// directory "/foo" were renamed. The event you receive is a
	// special event: the path for the event is the original path
	// you specified, the flag RootChanged is set and event ID is
	// zero. RootChanged events are useful to indicate that you
	// should rescan a particular hierarchy because it changed
	// completely (as opposed to the things inside of it changing).
	// If you want to track the current location of a directory, it
	// is best to open the directory before creating the stream so
	// that you have a file descriptor for it and can issue an
	// F_GETPATH fcntl() to find the current path.
	WatchRoot = CreateFlags(C.kFSEventStreamCreateFlagWatchRoot)

	// IgnoreSelf doesn't send events triggered by the current process (macOS 10.6+).
	//
	// Don't send events that were triggered by the current process.
	// This is useful for reducing the volume of events that are
	// sent. It is only useful if your process might modify the file
	// system hierarchy beneath the path(s) being monitored. Note:
	// this has no effect on historical events, i.e., those
	// delivered before the HistoryDone sentinel event.
	IgnoreSelf = CreateFlags(C.kFSEventStreamCreateFlagIgnoreSelf)

	// FileEvents sends events about individual files, generating significantly
	// more events (macOS 10.7+) than directory level notifications.
	FileEvents = CreateFlags(C.kFSEventStreamCreateFlagFileEvents)
)

// EventFlags passed to the FSEventStreamCallback function.
// These correspond directly to the flags as described here:
// https://developer.apple.com/documentation/coreservices/1455361-fseventstreameventflags
type EventFlags uint32

const (
	// MustScanSubDirs indicates that events were coalesced hierarchically.
	//
	// Your application must rescan not just the directory given in
	// the event, but all its children, recursively. This can happen
	// if there was a problem whereby events were coalesced
	// hierarchically. For example, an event in /Users/jsmith/Music
	// and an event in /Users/jsmith/Pictures might be coalesced
	// into an event with this flag set and path=/Users/jsmith. If
	// this flag is set you may be able to get an idea of whether
	// the bottleneck happened in the kernel (less likely) or in
	// your client (more likely) by checking for the presence of the
	// informational flags UserDropped or KernelDropped.
	MustScanSubDirs EventFlags = EventFlags(C.kFSEventStreamEventFlagMustScanSubDirs)

	// KernelDropped or UserDropped may be set in addition
	// to the MustScanSubDirs flag to indicate that a problem
	// occurred in buffering the events (the particular flag set
	// indicates where the problem occurred) and that the client
	// must do a full scan of any directories (and their
	// subdirectories, recursively) being monitored by this stream.
	// If you asked to monitor multiple paths with this stream then
	// you will be notified about all of them. Your code need only
	// check for the MustScanSubDirs flag; these flags (if present)
	// only provide information to help you diagnose the problem.
	KernelDropped = EventFlags(C.kFSEventStreamEventFlagKernelDropped)

	// UserDropped is related to UserDropped above.
	UserDropped = EventFlags(C.kFSEventStreamEventFlagUserDropped)

	// EventIDsWrapped indicates the 64-bit event ID counter wrapped around.
	//
	// If EventIdsWrapped is set, it means
	// the 64-bit event ID counter wrapped around. As a result,
	// previously-issued event ID's are no longer valid
	// for the EventID field when using EventStream.Resume.
	EventIDsWrapped = EventFlags(C.kFSEventStreamEventFlagEventIdsWrapped)

	// HistoryDone is a sentinel event when retrieving events with EventStream.Resume.
	//
	// Denotes a sentinel event sent to mark the end of the
	// "historical" events sent as a result of specifying
	// EventStream.Resume.
	//
	// After sending all the "historical" events that occurred before now,
	// an event will be sent with the HistoryDone flag set. The client
	// should ignore the path supplied in that event.
	HistoryDone = EventFlags(C.kFSEventStreamEventFlagHistoryDone)

	// RootChanged indicates a change to a directory along the path being watched.
	//
	// Denotes a special event sent when there is a change to one of
	// the directories along the path to one of the directories you
	// asked to watch. When this flag is set, the event ID is zero
	// and the path corresponds to one of the paths you asked to
	// watch (specifically, the one that changed). The path may no
	// longer exist because it or one of its parents was deleted or
	// renamed. Events with this flag set will only be sent if you
	// passed the flag WatchRoot when you created the stream.
	RootChanged = EventFlags(C.kFSEventStreamEventFlagRootChanged)

	// Mount for a volume mounted underneath the path being monitored.
	//
	// Denotes a special event sent when a volume is mounted
	// underneath one of the paths being monitored. The path in the
	// event is the path to the newly-mounted volume. You will
	// receive one of these notifications for every volume mount
	// event inside the kernel (independent of DiskArbitration).
	// Beware that a newly-mounted volume could contain an
	// arbitrarily large directory hierarchy. Avoid pitfalls like
	// triggering a recursive scan of a non-local filesystem, which
	// you can detect by checking for the absence of the MNT_LOCAL
	// flag in the f_flags returned by statfs(). Also be aware of
	// the MNT_DONTBROWSE flag that is set for volumes which should
	// not be displayed by user interface elements.
	Mount = EventFlags(C.kFSEventStreamEventFlagMount)

	// Unmount event occurs after a volume is unmounted.
	//
	// Denotes a special event sent when a volume is unmounted
	// underneath one of the paths being monitored. The path in the
	// event is the path to the directory from which the volume was
	// unmounted. You will receive one of these notifications for
	// every volume unmount event inside the kernel. This is not a
	// substitute for the notifications provided by the
	// DiskArbitration framework; you only get notified after the
	// unmount has occurred. Beware that unmounting a volume could
	// uncover an arbitrarily large directory hierarchy, although
	// macOS never does that.
	Unmount = EventFlags(C.kFSEventStreamEventFlagUnmount)

	// The following flags are only set when using FileEvents.

	// ItemCreated indicates that a file or directory has been created.
	ItemCreated = EventFlags(C.kFSEventStreamEventFlagItemCreated)

	// ItemRemoved indicates that a file or directory has been removed.
	ItemRemoved = EventFlags(C.kFSEventStreamEventFlagItemRemoved)

	// ItemInodeMetaMod indicates that a file or directory's metadata has has been modified.
	ItemInodeMetaMod = EventFlags(C.kFSEventStreamEventFlagItemInodeMetaMod)

	// ItemRenamed indicates that a file or directory has been renamed.
	// TODO is there any indication what it might have been renamed to?
	ItemRenamed = EventFlags(C.kFSEventStreamEventFlagItemRenamed)

	// ItemModified indicates that a file has been modified.
	ItemModified = EventFlags(C.kFSEventStreamEventFlagItemModified)

	// ItemFinderInfoMod indicates the the item's Finder information has been
	// modified.
	// TODO the above is just a guess.
	ItemFinderInfoMod = EventFlags(C.kFSEventStreamEventFlagItemFinderInfoMod)

	// ItemChangeOwner indicates that the file has changed ownership.
	ItemChangeOwner = EventFlags(C.kFSEventStreamEventFlagItemChangeOwner)

	// ItemXattrMod indicates that the files extended attributes have changed.
	ItemXattrMod = EventFlags(C.kFSEventStreamEventFlagItemXattrMod)

	// ItemIsFile indicates that the item is a file.
	ItemIsFile = EventFlags(C.kFSEventStreamEventFlagItemIsFile)

	// ItemIsDir indicates that the item is a directory.
	ItemIsDir = EventFlags(C.kFSEventStreamEventFlagItemIsDir)

	// ItemIsSymlink indicates that the item is a symbolic link.
	ItemIsSymlink = EventFlags(C.kFSEventStreamEventFlagItemIsSymlink)
)

const (
	nullCFStringRef = C.CFStringRef(0)
	nullCFUUIDRef   = C.CFUUIDRef(0)

	// eventIDSinceNow is a sentinel to begin watching events "since now".
	eventIDSinceNow = uint64(C.kFSEventStreamEventIdSinceNow)
)

// GetDeviceUUID retrieves the UUID required to identify an EventID
// in the FSEvents database
func GetDeviceUUID(deviceID int32) string {
	uuid := C.FSEventsCopyUUIDForDevice(C.dev_t(deviceID))
	if uuid == nullCFUUIDRef {
		return ""
	}
	return cfStringToGoString(C.CFUUIDCreateString(C.kCFAllocatorDefault, uuid))
}

// LatestEventID returns the most recently generated event ID, system-wide.
func LatestEventID() uint64 {
	return uint64(C.FSEventsGetCurrentEventId())
}

// arguments are released by C at the end of the callback. Ensure copies
// are made if data is expected to persist beyond this function ending.
//
//export fsevtCallback
func fsevtCallback(stream C.FSEventStreamRef, info uintptr, numEvents C.size_t, cpaths **C.char, cflags *C.FSEventStreamEventFlags, cids *C.FSEventStreamEventId) {
	l := int(numEvents)
	events := make([]Event, l)

	es := registry.Get(info)
	if es == nil {
		log.Printf("failed to retrieve registry %d", info)
		return
	}
	// These slices are backed by C data. Ensure data is copied out
	// if it expected to exist outside of this function.
	paths := (*[1 << 30]*C.char)(unsafe.Pointer(cpaths))[:l:l]
	ids := (*[1 << 30]C.FSEventStreamEventId)(unsafe.Pointer(cids))[:l:l]
	flags := (*[1 << 30]C.FSEventStreamEventFlags)(unsafe.Pointer(cflags))[:l:l]
	for i := range events {
		events[i] = Event{
			Path:  C.GoString(paths[i]),
			Flags: EventFlags(flags[i]),
			ID:    uint64(ids[i]),
		}
		es.EventID = uint64(ids[i])
	}

	es.Events <- events
}

type fsDispatchQueueRef C.dispatch_queue_t

// fsEventStreamRef wraps C.FSEventStreamRef
type fsEventStreamRef C.FSEventStreamRef

// getStreamRefEventID retrieves the last EventID from the ref
func getStreamRefEventID(f fsEventStreamRef) uint64 {
	return uint64(C.FSEventStreamGetLatestEventId(f))
}

// getStreamRefDeviceID retrieves the device ID the stream is watching
func getStreamRefDeviceID(f fsEventStreamRef) int32 {
	return int32(C.FSEventStreamGetDeviceBeingWatched(f))
}

// getStreamRefDescription retrieves debugging description information
// about the StreamRef
func getStreamRefDescription(f fsEventStreamRef) string {
	return cfStringToGoString(C.FSEventStreamCopyDescription(f))
}

// getStreamRefPaths returns a copy of the paths being watched by
// this stream
func getStreamRefPaths(f fsEventStreamRef) []string {
	arr := C.FSEventStreamCopyPathsBeingWatched(f)
	l := cfArrayLen(arr)

	ss := make([]string, l)
	for i := range ss {
		void := C.CFArrayGetValueAtIndex(arr, C.CFIndex(i))
		ss[i] = cfStringToGoString(C.CFStringRef(void))
	}
	return ss
}

func cfStringToGoString(cfs C.CFStringRef) string {
	if cfs == nullCFStringRef {
		return ""
	}
	cfStr := copyCFString(cfs)
	length := C.CFStringGetLength(cfStr)
	if length == 0 {
		// short-cut for empty strings
		return ""
	}
	cfRange := C.CFRange{0, length}
	enc := C.CFStringEncoding(C.kCFStringEncodingUTF8)
	// first find the buffer size necessary
	var usedBufLen C.CFIndex
	if C.CFStringGetBytes(cfStr, cfRange, enc, 0, C.false, nil, 0, &usedBufLen) == 0 {
		return ""
	}

	bs := make([]byte, usedBufLen)
	buf := (*C.UInt8)(unsafe.Pointer(&bs[0]))
	if C.CFStringGetBytes(cfStr, cfRange, enc, 0, C.false, buf, usedBufLen, nil) == 0 {
		return ""
	}

	// Create a string (byte array) backed by C byte array
	header := (*reflect.SliceHeader)(unsafe.Pointer(&bs))
	strHeader := &reflect.StringHeader{
		Data: header.Data,
		Len:  header.Len,
	}
	return *(*string)(unsafe.Pointer(strHeader))
}

// copyCFString makes an immutable copy of a string with CFStringCreateCopy.
func copyCFString(cfs C.CFStringRef) C.CFStringRef {
	return C.CFStringCreateCopy(C.kCFAllocatorDefault, cfs)
}

// EventIDForDeviceBeforeTime returns an event ID before a given time.
func EventIDForDeviceBeforeTime(dev int32, before time.Time) uint64 {
	tm := C.CFAbsoluteTime(before.Unix())
	return uint64(C.FSEventsGetLastEventIdForDeviceBeforeTime(C.dev_t(dev), tm))
}

// createPaths accepts the user defined set of paths and returns FSEvents
// compatible array of paths
func createPaths(paths []string) (C.CFArrayRef, error) {
	cPaths := C.ArrayCreateMutable(C.int(len(paths)))
	var errs []error
	for _, path := range paths {
		p, err := filepath.Abs(path)
		if err != nil {
			// hack up some reporting errors, but don't prevent execution
			// because of them
			errs = append(errs, err)
		}
		str := makeCFString(p)
		C.CFArrayAppendValue(C.CFMutableArrayRef(cPaths), unsafe.Pointer(str))
	}
	var err error
	if len(errs) > 0 {
		err = fmt.Errorf("%q", errs)
	}
	return cPaths, err
}

// makeCFString makes an immutable string with CFStringCreateWithCString.
func makeCFString(str string) C.CFStringRef {
	s := C.CString(str)
	defer C.free(unsafe.Pointer(s))
	return C.CFStringCreateWithCString(C.kCFAllocatorDefault, s, C.kCFStringEncodingUTF8)
}

// CFArrayLen retrieves the length of CFArray type
// See https://developer.apple.com/library/mac/documentation/CoreFoundation/Reference/CFArrayRef/#//apple_ref/c/func/CFArrayGetCount
func cfArrayLen(ref C.CFArrayRef) int {
	// FIXME: this will probably crash on 32bit, untested
	// requires OS X v10.0
	return int(C.CFArrayGetCount(ref))
}

func setupStream(paths []string, flags CreateFlags, callbackInfo uintptr, eventID uint64, latency time.Duration, deviceID int32) fsEventStreamRef {
	cPaths, err := createPaths(paths)
	if err != nil {
		log.Printf("Error creating paths: %s", err)
	}
	defer C.CFRelease(C.CFTypeRef(cPaths))

	since := C.FSEventStreamEventId(eventID)
	context := C.FSEventStreamContext{}
	info := C.uintptr_t(callbackInfo)
	cfinv := C.CFTimeInterval(float64(latency) / float64(time.Second))

	var ref C.FSEventStreamRef
	if deviceID != 0 {
		ref = C.EventStreamCreateRelativeToDevice(&context, info,
			C.dev_t(deviceID), cPaths, since, cfinv,
			C.FSEventStreamCreateFlags(flags))
	} else {
		ref = C.EventStreamCreate(&context, info, cPaths, since,
			cfinv, C.FSEventStreamCreateFlags(flags))
	}

	return fsEventStreamRef(ref)
}

func (es *EventStream) start(paths []string, callbackInfo uintptr) error {

	since := eventIDSinceNow
	if es.Resume {
		since = es.EventID
	}

	es.stream = setupStream(paths, es.Flags, callbackInfo, since, es.Latency, es.Device)

	es.qref = fsDispatchQueueRef(C.dispatch_queue_create(nil, nil))
	C.FSEventStreamSetDispatchQueue(es.stream, es.qref)

	if C.FSEventStreamStart(es.stream) == 0 {
		// cleanup stream
		C.FSEventStreamInvalidate(es.stream)
		C.FSEventStreamRelease(es.stream)
		es.stream = nil

		C.DispatchQueueRelease(es.qref)
		es.qref = nil

		return fmt.Errorf("failed to start eventstream")
	}

	if !es.hasFinalizer {
		// TODO: There is no guarantee this run before program exit
		// and could result in panics at exit.
		runtime.SetFinalizer(es, finalizer)
		es.hasFinalizer = true
	}

	return nil
}

func finalizer(es *EventStream) {
	// If an EventStream is freed without Stop being called it will
	// cause a panic. This avoids that, and closes the stream instead.
	es.Stop()
}

// flush drains the event stream of undelivered events
func flush(stream fsEventStreamRef, sync bool) {
	if sync {
		C.FSEventStreamFlushSync(stream)
	} else {
		C.FSEventStreamFlushAsync(stream)
	}
}

// stop requests fsevents stops streaming events
func stop(stream fsEventStreamRef, qref fsDispatchQueueRef) {
	C.FSEventStreamStop(stream)
	C.FSEventStreamInvalidate(stream)
	C.FSEventStreamRelease(stream)
	C.DispatchQueueRelease(qref)
}
