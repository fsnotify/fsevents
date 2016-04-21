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
import "unsafe"

// EventIDSinceNow is a sentinel to begin watching events "since now".
const EventIDSinceNow = uint64(C.kFSEventStreamEventIdSinceNow + (1 << 64))

// arguments are released by C at the end of the callback. Ensure copies
// are made if data is expected to persist beyond this function ending.
//
//export fsevtCallback
func fsevtCallback(stream C.FSEventStreamRef, info uintptr, numEvents C.size_t, cpaths **C.char, cflags *C.FSEventStreamEventFlags, cids *C.FSEventStreamEventId) {
	l := int(numEvents)
	events := make([]Event, l)

	es := registry.Get(info)
	if es == nil {
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

// FSEventStreamRef wraps C.FSEventStreamRef
type FSEventStreamRef C.FSEventStreamRef

// CFRunLoopRef wraps C.CFRunLoopRef
type CFRunLoopRef C.CFRunLoopRef
