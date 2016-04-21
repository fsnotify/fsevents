// +buid darwin

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
*/
import "C"
import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"
)

// EventIDSinceNow is a sentinel to begin watching events "since now".
const EventIDSinceNow = uint64(C.kFSEventStreamEventIdSinceNow + (1 << 64))

func lastEventID() uint64 {
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
		fmt.Println("round", i)
		events[i] = Event{
			Path:  C.GoString(paths[i]),
			Flags: EventFlags(flags[i]),
			ID:    uint64(ids[i]),
		}
		es.EventID = uint64(ids[i])
		fmt.Printf("event % #v\n", events[i])
	}

	es.Events <- events
}

// FSEventStreamRef wraps C.FSEventStreamRef
type FSEventStreamRef C.FSEventStreamRef

// CFRunLoopRef wraps C.CFRunLoopRef
type CFRunLoopRef C.CFRunLoopRef

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
		cpath := C.CString(p)
		defer C.free(unsafe.Pointer(cpath))

		str := C.CFStringCreateWithCString(nil, cpath, C.kCFStringEncodingUTF8)
		C.CFArrayAppendValue(cPaths, unsafe.Pointer(str))
	}
	var err error
	if len(errs) > 0 {
		err = fmt.Errorf("%q", errs)
	}
	return cPaths, err
}

func (es *EventStream) start(paths []string, callbackInfo uintptr) {
	cPaths, err := createPaths(paths)
	if err != nil {
		log.Printf("Error creating paths: %s", err)
	}
	defer C.CFRelease(C.CFTypeRef(cPaths))

	since := C.FSEventStreamEventId(EventIDSinceNow)
	if es.Resume {
		since = C.FSEventStreamEventId(es.EventID)
	}

	context := C.FSEventStreamContext{}
	info := C.uintptr_t(callbackInfo)
	latency := C.CFTimeInterval(float64(es.Latency) / float64(time.Second))
	if es.Device != 0 {
		ref := C.EventStreamCreateRelativeToDevice(&context, info, C.dev_t(es.Device), cPaths, since, latency, C.FSEventStreamCreateFlags(es.Flags))
		es.stream = FSEventStreamRef(ref)
	} else {
		ref := C.EventStreamCreate(&context, info, cPaths, since, latency, C.FSEventStreamCreateFlags(es.Flags))
		es.stream = FSEventStreamRef(ref)
	}

	go func() {
		runtime.LockOSThread()
		es.rlref = CFRunLoopRef(C.CFRunLoopGetCurrent())
		C.FSEventStreamScheduleWithRunLoop(es.stream, es.rlref, C.kCFRunLoopDefaultMode)
		C.FSEventStreamStart(es.stream)
		C.CFRunLoopRun()
	}()

	if !es.hasFinalizer {
		// TODO: There is no guarantee this run before program exit
		// and could result in panics at exit.
		runtime.SetFinalizer(es, finalizer)
		es.hasFinalizer = true
	}

}

func finalizer(es *EventStream) {
	// If an EventStream is freed without Stop being called it will
	// cause a panic. This avoids that, and closes the stream instead.
	es.Stop()
}

// flush drains the event stream of undelivered events
func flush(stream FSEventStreamRef, sync bool) {
	if sync {
		C.FSEventStreamFlushSync(stream)
	} else {
		C.FSEventStreamFlushAsync(stream)
	}
}

// stop requests fsevents stops streaming events
func stop(stream FSEventStreamRef, rlref CFRunLoopRef) {
	C.FSEventStreamStop(stream)
	C.FSEventStreamInvalidate(stream)
	C.FSEventStreamRelease(stream)
	C.CFRunLoopStop(rlref)
}
