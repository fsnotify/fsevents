package fsevents

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

	CreateFlagNone CreateFlags = 0
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

	Created
	Removed
	InodeMetaMod
	Renamed
	Modified
	FinderInfoMod
	ChangeOwner
	XattrMod
	IsFile
	IsDir
	IsSymlink

	EventFlagNone EventFlags = 0
)
