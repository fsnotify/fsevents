package fsevents

/*
#cgo LDFLAGS: -framework CoreServices
#include <CoreServices/CoreServices.h>
#include <sys/stat.h>

static CFArrayRef ArrayCreateMutable(int len) {
	return CFArrayCreateMutable(NULL, len, &kCFTypeArrayCallBacks);
}

extern void fsevtCallback(FSEventStreamRef p0, void * info, size_t p1, char** p2, FSEventStreamEventFlags* p3, FSEventStreamEventId* p4);

static FSEventStreamRef EventStreamCreateRelativeToDevice(FSEventStreamContext * context, dev_t dev, CFArrayRef paths, FSEventStreamEventId since, CFTimeInterval latency, FSEventStreamCreateFlags flags) {
	return FSEventStreamCreateRelativeToDevice(NULL, (FSEventStreamCallback) fsevtCallback, context, dev, paths, since, latency, flags);
}

static FSEventStreamRef EventStreamCreate(FSEventStreamContext * context, CFArrayRef paths, FSEventStreamEventId since, CFTimeInterval latency, FSEventStreamCreateFlags flags) {
	return FSEventStreamCreate(NULL, (FSEventStreamCallback) fsevtCallback, context, paths, since, latency, flags);
}
*/
import "C"
import "unsafe"
import "path/filepath"

// CreateFlags for creating a New stream.
type CreateFlags uint32

// kFSEventStreamCreateFlag...
const (
	// use CoreFoundation types instead of raw C types (disabled)
	useCFTypes CreateFlags = 1 << iota

	// NoDefer sends events on the leading edge (for interactive applications).
	// By default events are delivered after latency seconds (for background tasks).
	NoDefer

	// WatchRoot for a change to occur to a directory along the path being watched.
	WatchRoot

	// IgnoreSelf doesn't send events triggered by the current process (OS X 10.6+).
	IgnoreSelf

	// FileEvents sends events about individual files, generating significantly
	// more events (OS X 10.7+) than directory level notifications.
	FileEvents
)

// EventFlags passed to the FSEventStreamCallback function.
type EventFlags uint32

// kFSEventStreamEventFlag...
const (
	// MustScanSubDirs indicates that events were coalesced hierarchically.
	MustScanSubDirs EventFlags = 1 << iota
	// UserDropped or KernelDropped is set alongside MustScanSubDirs
	// to help diagnose the problem.
	UserDropped
	KernelDropped

	// EventIdsWrapped indicates the 64-bit event ID counter wrapped around.
	EventIdsWrapped

	// HistoryDone is a sentinel event when retrieving events sinceWhen.
	HistoryDone

	// RootChanged indicates a change to a directory along the path being watched.
	RootChanged

	// Mount for a volume mounted underneath the path being monitored.
	Mount
	// Unmount event occurs after a volume is unmounted.
	Unmount

	// The following flags are only set when using FileEvents.

	ItemCreated
	ItemRemoved
	ItemInodeMetaMod
	ItemRenamed
	ItemModified
	ItemFinderInfoMod
	ItemChangeOwner
	ItemXattrMod
	ItemIsFile
	ItemIsDir
	ItemIsSymlink
)

type FSEvent struct {
	Path  string
	Flags EventFlags
	Id    uint64
}

//export fsevtCallback
func fsevtCallback(stream C.FSEventStreamRef, info unsafe.Pointer, numEvents C.size_t, paths **C.char, flags *C.FSEventStreamEventFlags, ids *C.FSEventStreamEventId) {
	events := make([]FSEvent, int(numEvents))

	for i := 0; i < int(numEvents); i++ {
		cpaths := uintptr(unsafe.Pointer(paths)) + (uintptr(i) * unsafe.Sizeof(*paths))
		cpath := *(**C.char)(unsafe.Pointer(cpaths))

		cflags := uintptr(unsafe.Pointer(flags)) + (uintptr(i) * unsafe.Sizeof(*flags))
		cflag := *(*C.FSEventStreamEventFlags)(unsafe.Pointer(cflags))

		cids := uintptr(unsafe.Pointer(ids)) + (uintptr(i) * unsafe.Sizeof(*ids))
		cid := *(*C.FSEventStreamEventId)(unsafe.Pointer(cids))

		events[i] = FSEvent{Path: C.GoString(cpath), Flags: EventFlags(cflag), Id: uint64(cid)}
	}

	evtC := *(*chan []FSEvent)(info)
	evtC <- events
}

/*
	extern FSEventStreamRef FSEventStreamCreate(
		CFAllocatorRef allocator,
		FSEventStreamCallback callback,
		FSEventStreamContext *context,
		CFArrayRef pathsToWatch,
		FSEventStreamEventId sinceWhen,
		CFTimeInterval latency,
		FSEventStreamCreateFlags flags);

	typedef void ( *FSEventStreamCallback )(
		ConstFSEventStreamRef streamRef,
		void *clientCallBackInfo,
		size_t numEvents,
		void *eventPaths,
		const FSEventStreamEventFlags eventFlags[],
		const FSEventStreamEventId eventIds[]);
*/

func FSEventsLatestId() uint64 {
	return uint64(C.FSEventsGetCurrentEventId())
}

func DeviceForPath(pth string) int64 {
	cStat := C.struct_stat{}
	cPath := C.CString(pth)
	defer C.free(unsafe.Pointer(cPath))

	_ = C.lstat(cPath, &cStat)
	return int64(cStat.st_dev)
}

func GetIdForDeviceBeforeTime(dev, tm int64) uint64 {
	return uint64(C.FSEventsGetLastEventIdForDeviceBeforeTime(C.dev_t(dev), C.CFAbsoluteTime(tm)))
}

func FSEventsSince(paths []string, dev int64, since uint64) []FSEvent {
	cPaths := C.ArrayCreateMutable(C.int(len(paths)))
	defer C.CFRelease(C.CFTypeRef(cPaths))

	for _, p := range paths {
		p, _ = filepath.Abs(p)
		cpath := C.CString(p)
		defer C.free(unsafe.Pointer(cpath))

		str := C.CFStringCreateWithCString(nil, cpath, C.kCFStringEncodingUTF8)
		C.CFArrayAppendValue(cPaths, unsafe.Pointer(str))
	}

	if since == 0 {
		/* If since == 0 is passed to FSEventStreamCreate it will mean 'since the beginning of time'.
		We remap to 'now'. */
		since = C.kFSEventStreamEventIdSinceNow + (1 << 64)
	}

	evtC := make(chan []FSEvent)
	context := C.FSEventStreamContext{info: unsafe.Pointer(&evtC)}

	latency := C.CFTimeInterval(1.0)
	var stream C.FSEventStreamRef
	if dev != 0 {
		stream = C.EventStreamCreateRelativeToDevice(&context, C.dev_t(dev), cPaths, C.FSEventStreamEventId(since), latency, 0)
	} else {
		stream = C.EventStreamCreate(&context, cPaths, C.FSEventStreamEventId(since), latency, 0)
	}

	rlref := C.CFRunLoopGetCurrent()

	go func() {
		/* Schedule the stream on the runloop, then run the runloop concurrently with starting/flushing/stopping the stream */
		C.FSEventStreamScheduleWithRunLoop(stream, rlref, C.kCFRunLoopDefaultMode)
		go func() {
			C.CFRunLoopRun()
		}()
		C.FSEventStreamStart(stream)
		C.FSEventStreamFlushSync(stream)
		C.FSEventStreamStop(stream)
		C.FSEventStreamInvalidate(stream)
		C.FSEventStreamRelease(stream)
		C.CFRunLoopStop(rlref)
		close(evtC)
	}()

	var events []FSEvent
	for evts := range evtC {
		events = append(events, evts...)
	}

	return events
}

type FSEventStream struct {
	stream C.FSEventStreamRef
	rlref  C.CFRunLoopRef
	C      chan []FSEvent
}

func NewEventStream(paths []string, since uint64, flags CreateFlags) *FSEventStream {
	cPaths := C.ArrayCreateMutable(C.int(len(paths)))
	defer C.CFRelease(C.CFTypeRef(cPaths))

	for _, p := range paths {
		p, _ = filepath.Abs(p)
		cpath := C.CString(p)
		defer C.free(unsafe.Pointer(cpath))

		str := C.CFStringCreateWithCString(nil, cpath, C.kCFStringEncodingUTF8)
		C.CFArrayAppendValue(cPaths, unsafe.Pointer(str))
	}

	if since == 0 {
		/* If since == 0 is passed to FSEventStreamCreate it will mean 'since the beginning of time'.
		We remap to 'now'. */
		since = C.kFSEventStreamEventIdSinceNow + (1 << 64)
	}

	es := FSEventStream{C: make(chan []FSEvent)}
	context := C.FSEventStreamContext{info: unsafe.Pointer(&es.C)}

	latency := C.CFTimeInterval(1.0)
	es.stream = C.EventStreamCreate(&context, cPaths, C.FSEventStreamEventId(since), latency, C.FSEventStreamCreateFlags(flags))

	go func() {
		es.rlref = C.CFRunLoopGetCurrent()
		C.FSEventStreamScheduleWithRunLoop(es.stream, es.rlref, C.kCFRunLoopDefaultMode)
		C.FSEventStreamStart(es.stream)
		C.CFRunLoopRun()
	}()

	return &es
}

func (es *FSEventStream) Flush() {
	C.FSEventStreamFlushSync(es.stream)
}

func (es *FSEventStream) Stop() {
	C.FSEventStreamStop(es.stream)
	C.FSEventStreamInvalidate(es.stream)
	C.FSEventStreamRelease(es.stream)
	C.CFRunLoopStop(es.rlref)
	close(es.C)
}
