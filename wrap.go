//go:build darwin

package fsevents

import (
	"fmt"
	"log"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

type CreateFlags uint32

const (
	NoDefer    CreateFlags = 0x00000002
	WatchRoot  CreateFlags = 0x00000004
	IgnoreSelf CreateFlags = 0x00000008
	FileEvents CreateFlags = 0x00000010
)

type EventFlags uint32

const (
	MustScanSubDirs   EventFlags = 0x00000001
	KernelDropped     EventFlags = 0x00000002
	UserDropped       EventFlags = 0x00000004
	EventIDsWrapped   EventFlags = 0x00000008
	HistoryDone       EventFlags = 0x00000010
	RootChanged       EventFlags = 0x00000020
	Mount             EventFlags = 0x00000040
	Unmount           EventFlags = 0x00000080
	ItemCreated       EventFlags = 0x00000100
	ItemRemoved       EventFlags = 0x00000200
	ItemInodeMetaMod  EventFlags = 0x00000400
	ItemRenamed       EventFlags = 0x00000800
	ItemModified      EventFlags = 0x00001000
	ItemFinderInfoMod EventFlags = 0x00002000
	ItemChangeOwner   EventFlags = 0x00004000
	ItemXattrMod      EventFlags = 0x00008000
	ItemIsFile        EventFlags = 0x00010000
	ItemIsDir         EventFlags = 0x00020000
	ItemIsSymlink     EventFlags = 0x00040000
)

const (
	eventIDSinceNow = ^uint64(0) // kFSEventStreamEventIdSinceNow
)

var (
	// CoreServices function pointers
	fseventsCreateRelativeToDevice            uintptr
	fseventsCreate                            uintptr
	fseventsStart                             uintptr
	fseventsStop                              uintptr
	fseventsInvalidate                        uintptr
	fseventsRelease                           uintptr
	fseventsGetLatestEventID                  uintptr
	fseventsGetDeviceBeingWatched             uintptr
	fseventsCopyDescription                   uintptr
	fseventsCopyPaths                         uintptr
	fseventsFlushAsync                        uintptr
	fseventsFlushSync                         uintptr
	fseventsSetDispatchQueue                  uintptr
	fseventsCopyUUIDForDevice                 uintptr
	fseventsGetLastEventIDForDeviceBeforeTime uintptr

	// CoreFoundation function pointers
	cfRelease                 uintptr
	cfStringCreateWithCString uintptr
	cfURLCreateWithString     uintptr
	cfStringGetCStringPtr     uintptr
	cfURLGetString            uintptr
	cfStringGetLength         uintptr
	cfStringGetCString        uintptr
	cfArrayGetCount           uintptr
	cfArrayGetValueAtIndex    uintptr
	cfArrayCreateMutable      uintptr
	cfArrayAppendValue        uintptr
	cfUUIDCreateString        uintptr
	cfAbsoluteTime            uintptr

	// Dispatch function pointers
	dispatchQueueCreate uintptr
	dispatchRelease     uintptr
)

const (
	kCFStringEncodingUTF8 = 0x08000100
	kCFAllocatorDefault   = 0
)

type (
	fsEventStreamRef   uintptr
	fsDispatchQueueRef uintptr
	CFStringRef        uintptr
	CFURLRef           uintptr
	CFArrayRef         uintptr
)

func init() {
	// Load CoreServices framework
	coreServices, err := purego.Dlopen("/System/Library/Frameworks/CoreServices.framework/CoreServices", purego.RTLD_LAZY)
	if err != nil {
		panic(err)
	}

	// Register CoreServices functions
	fseventsCreateRelativeToDevice, _ = purego.Dlsym(coreServices, "FSEventStreamCreateRelativeToDevice")
	fseventsCreateRelativeToDevice, _ = purego.Dlsym(coreServices, "FSEventStreamCreateRelativeToDevice")
	fseventsCreate, _ = purego.Dlsym(coreServices, "FSEventStreamCreate")
	fseventsStart, _ = purego.Dlsym(coreServices, "FSEventStreamStart")
	fseventsStop, _ = purego.Dlsym(coreServices, "FSEventStreamStop")
	fseventsInvalidate, _ = purego.Dlsym(coreServices, "FSEventStreamInvalidate")
	fseventsRelease, _ = purego.Dlsym(coreServices, "FSEventStreamRelease")
	fseventsGetLatestEventID, _ = purego.Dlsym(coreServices, "FSEventStreamGetLatestEventId")
	fseventsGetDeviceBeingWatched, _ = purego.Dlsym(coreServices, "FSEventStreamGetDeviceBeingWatched")
	fseventsCopyDescription, _ = purego.Dlsym(coreServices, "FSEventStreamCopyDescription")
	fseventsCopyPaths, _ = purego.Dlsym(coreServices, "FSEventStreamCopyPathsBeingWatched")
	fseventsFlushAsync, _ = purego.Dlsym(coreServices, "FSEventStreamFlushAsync")
	fseventsFlushSync, _ = purego.Dlsym(coreServices, "FSEventStreamFlushSync")
	fseventsSetDispatchQueue, _ = purego.Dlsym(coreServices, "FSEventStreamSetDispatchQueue")
	fseventsCopyUUIDForDevice, _ = purego.Dlsym(coreServices, "FSEventsCopyUUIDForDevice")
	fseventsGetLastEventIDForDeviceBeforeTime, _ = purego.Dlsym(coreServices, "FSEventsGetLastEventIDForDeviceBeforeTime")

	// Register CoreFoundation functions
	cfRelease, _ = purego.Dlsym(coreServices, "CFRelease")
	cfStringCreateWithCString, _ = purego.Dlsym(coreServices, "CFStringCreateWithCString")
	cfURLCreateWithString, _ = purego.Dlsym(coreServices, "CFURLCreateWithString")
	cfStringGetCStringPtr, _ = purego.Dlsym(coreServices, "CFStringGetCStringPtr")
	cfURLGetString, _ = purego.Dlsym(coreServices, "CFURLGetString")
	cfStringGetLength, _ = purego.Dlsym(coreServices, "CFStringGetLength")
	cfStringGetCString, _ = purego.Dlsym(coreServices, "CFStringGetCString")
	cfArrayGetCount, _ = purego.Dlsym(coreServices, "CFArrayGetCount")
	cfArrayGetValueAtIndex, _ = purego.Dlsym(coreServices, "CFArrayGetValueAtIndex")
	cfArrayCreateMutable, _ = purego.Dlsym(coreServices, "CFArrayCreateMutable")
	cfArrayAppendValue, _ = purego.Dlsym(coreServices, "CFArrayAppendValue")
	cfUUIDCreateString, _ = purego.Dlsym(coreServices, "CFUUIDCreateString")
	cfAbsoluteTime, _ = purego.Dlsym(coreServices, "CFAbsoluteTimeGetCurrent")

	// Register Dispatch functions
	dispatch, err := purego.Dlopen("/usr/lib/system/libdispatch.dylib", purego.RTLD_LAZY)
	if err != nil {
		panic(err)
	}
	dispatchQueueCreate, _ = purego.Dlsym(dispatch, "dispatch_queue_create")
	dispatchRelease, _ = purego.Dlsym(dispatch, "dispatch_release")
}

func cfReleaseCall(ref interface{}) {
	if _, ok := ref.(uintptr); ok {
		purego.SyscallN(cfRelease, ref.(uintptr))
	}
}

func cStringToGoString(cstr uintptr) string {
	if cstr == 0 {
		return ""
	}
	// Find the length of the null-terminated C string
	length := 0
	for {
		// Read byte at offset `length` from the pointer
		if *(*byte)(unsafe.Pointer(cstr + uintptr(length))) == 0 {
			break
		}
		length++
	}
	if length == 0 {
		return ""
	}
	// Convert the C string to a Go string using unsafe.Slice
	data := unsafe.Slice((*byte)(unsafe.Pointer(cstr)), length)
	return string(data)
}

// goStringToCFString converts a Go string to a CFStringRef
func goStringToCFString(s string) CFStringRef {
	// Convert to null-terminated byte slice
	bytes := append([]byte(s), 0) // Safe allocation, null-terminated
	cStr := unsafe.Pointer(&bytes[0])
	ret, _, _ := purego.SyscallN(cfStringCreateWithCString,
		0,             // allocator (NULL)
		uintptr(cStr), // C string pointer
		kCFStringEncodingUTF8,
	)
	return CFStringRef(ret)
}

// goStringToCFURL converts a Go string to a CFURLRef
func goStringToCFURL(s string) CFURLRef {
	urlStr := goStringToCFString(s)
	if urlStr == 0 {
		return 0
	}
	ret, _, _ := purego.SyscallN(cfURLCreateWithString,
		0, // allocator (NULL)
		uintptr(urlStr),
		0, // baseURL (NULL)
	)
	cfReleaseCall(uintptr(urlStr))
	return CFURLRef(ret)
}

// cfStringToGoString converts a CFStringRef to a Go string
func cfStringToGoString(ref CFStringRef) string {
	if ref == 0 {
		return ""
	}

	// Get the length of the string in UTF-16 code units
	length, _, _ := purego.SyscallN(cfStringGetLength, uintptr(ref))
	if length == 0 {
		return ""
	}

	// Estimate buffer size: assume max 3 bytes per UTF-16 unit (worst-case UTF-8)
	// Add 1 for null terminator
	maxBytes := (length * 5) + 1
	buffer := make([]byte, maxBytes)

	// Copy the string into the buffer as UTF-8
	success, _, _ := purego.SyscallN(cfStringGetCString,
		uintptr(ref),                        // CFStringRef
		uintptr(unsafe.Pointer(&buffer[0])), // Buffer
		maxBytes,                            // Buffer size
		kCFStringEncodingUTF8,               // Encoding
	)

	if success == 0 {
		return "" // Failed to convert, return empty string
	}

	// Find the null terminator to determine actual length
	for i, b := range buffer {
		if b == 0 {
			return string(buffer[:i])
		}
	}
	return string(buffer[:maxBytes-1]) // Fallback, assume full buffer minus null
}

func cfURLToGoString(ref CFURLRef) string {
	if ref == 0 {
		return ""
	}
	urlStrRef, _, _ := purego.SyscallN(cfURLGetString, uintptr(ref))
	return cfStringToGoString(CFStringRef(urlStrRef))
}

// Callback function for FSEvents
func callback(stream uintptr, info uintptr, numEvents int, paths uintptr, flags uintptr, ids uintptr) {
	es := registry.Get(info)
	if es == nil {
		log.Printf("failed to retrieve registry %d", info)
		return
	}

	l := numEvents
	events := make([]Event, l)

	pathSlice := (*[1 << 30]uintptr)(unsafe.Pointer(paths))[:l:l]
	flagSlice := (*[1 << 30]uint32)(unsafe.Pointer(flags))[:l:l]
	idSlice := (*[1 << 30]uint64)(unsafe.Pointer(ids))[:l:l]

	for i := 0; i < l; i++ {
		path := cStringToGoString(pathSlice[i])
		events[i] = Event{
			Path:  path,
			Flags: EventFlags(flagSlice[i]),
			ID:    idSlice[i],
		}
		es.EventID = idSlice[i]
	}

	es.Events <- events
}

func createPaths(paths []string) (CFArrayRef, error) {
	cfArray, _, _ := purego.SyscallN(cfArrayCreateMutable, 0, uintptr(len(paths)), 0)
	var errs []error
	for _, path := range paths {
		p, err := filepath.Abs(path)
		if err != nil {
			errs = append(errs, err)
		}
		cfStr := goStringToCFString(p)
		purego.SyscallN(cfArrayAppendValue, cfArray, uintptr(cfStr))
	}
	var err error
	if len(errs) > 0 {
		err = fmt.Errorf("%q", errs)
	}
	return CFArrayRef(cfArray), err
}

func setupStream(paths []string, flags CreateFlags, callbackInfo uintptr, eventID uint64, latency time.Duration, deviceID int32) fsEventStreamRef {
	cPaths, err := createPaths(paths)
	if err != nil {
		log.Printf("Error creating paths: %s", err)
	}
	defer purego.SyscallN(cfRelease, uintptr(cPaths))

	var context [5]uintptr // FSEventStreamContext: {version, info, retain, release, copyDescription}
	context[1] = callbackInfo

	since := eventID
	cfinv := float64(latency) / float64(time.Second)
	cb := purego.NewCallback(callback)

	var ref uintptr
	if deviceID != 0 {
		ref, _, _ = purego.SyscallN(fseventsCreateRelativeToDevice,
			0, cb, uintptr(unsafe.Pointer(&context)), uintptr(deviceID), uintptr(cPaths), uintptr(since), uintptr(unsafe.Pointer(&cfinv)), uintptr(flags))
	} else {
		ref, _, _ = purego.SyscallN(fseventsCreate,
			0, cb, uintptr(unsafe.Pointer(&context)), uintptr(cPaths), uintptr(since), uintptr(unsafe.Pointer(&cfinv)), uintptr(flags))
	}

	return fsEventStreamRef(ref)
}

func (es *EventStream) start(paths []string, cbInfo uintptr) error {
	since := eventIDSinceNow
	if es.Resume {
		since = es.EventID
	}

	es.stream = setupStream(paths, es.Flags, cbInfo, since, es.Latency, es.Device)

	res, _, _ := purego.SyscallN(dispatchQueueCreate, 0, 0)
	es.qref = fsDispatchQueueRef(res)
	purego.SyscallN(fseventsSetDispatchQueue, uintptr(es.stream), uintptr(es.qref))

	if res, _, _ := purego.SyscallN(fseventsStart, uintptr(es.stream)); res == 0 {
		purego.SyscallN(fseventsInvalidate, uintptr(es.stream))
		purego.SyscallN(fseventsRelease, uintptr(es.stream))
		purego.SyscallN(dispatchRelease, uintptr(es.qref))
		return fmt.Errorf("failed to start eventstream")
	}

	return nil
}

func flush(stream fsEventStreamRef, sync bool) {
	if stream == 0 {
		return
	}

	if sync {
		purego.SyscallN(fseventsFlushSync, uintptr(stream))
	} else {
		purego.SyscallN(fseventsFlushAsync, uintptr(stream))
	}
}

func stop(stream fsEventStreamRef, qref fsDispatchQueueRef) {
	if stream == 0 {
		return
	}

	purego.SyscallN(fseventsStop, uintptr(stream))
	purego.SyscallN(fseventsInvalidate, uintptr(stream))
	purego.SyscallN(fseventsRelease, uintptr(stream))
	purego.SyscallN(dispatchRelease, uintptr(qref))
}

func CFArrayLen(ref CFArrayRef) int {
	if ref == 0 {
		return 0
	}
	count, _, _ := purego.SyscallN(cfArrayGetCount, uintptr(ref))
	return int(count)
}

// Additional helper functions
func LatestEventID() uint64 {
	res, _, _ := purego.SyscallN(fseventsGetLatestEventID, 0)
	return uint64(res)
}

// EventIDForDeviceBeforeTime returns an event ID before a given time.
func EventIDForDeviceBeforeTime(dev int32, before time.Time) uint64 {
	tm, _, _ := purego.SyscallN(cfAbsoluteTime, uintptr(before.Unix()))
	eventID, _, _ := purego.SyscallN(fseventsGetLastEventIDForDeviceBeforeTime, uintptr(dev), tm)
	return uint64(eventID)
}

// GetDeviceUUID retrieves the UUID required to identify an EventID
// in the FSEvents database
func GetDeviceUUID(deviceID int32) string {
	uuid, _, _ := purego.SyscallN(fseventsCopyUUIDForDevice, uintptr(deviceID))
	if uuid == 0 {
		return ""
	}
	uuidStr, _, _ := purego.SyscallN(cfUUIDCreateString, kCFAllocatorDefault, uintptr(uuid))
	return cfStringToGoString(CFStringRef(uuidStr))
}

func getStreamRefEventID(stream fsEventStreamRef) uint64 {
	res, _, _ := purego.SyscallN(fseventsGetLatestEventID, uintptr(stream))
	return uint64(res)
}

func getStreamRefDeviceID(stream fsEventStreamRef) int32 {
	res, _, _ := purego.SyscallN(fseventsGetDeviceBeingWatched, uintptr(stream))
	return int32(res)
}

func getStreamRefDescription(stream fsEventStreamRef) string {
	cfStr, _, _ := purego.SyscallN(fseventsCopyDescription, uintptr(stream))
	defer purego.SyscallN(cfRelease, cfStr)
	return cfStringToGoString(CFStringRef(cfStr))
}

func getStreamRefPaths(stream fsEventStreamRef) []string {
	arr, _, _ := purego.SyscallN(fseventsCopyPaths, uintptr(stream))
	defer purego.SyscallN(cfRelease, arr)
	l, _, _ := purego.SyscallN(cfArrayGetCount, arr)
	ss := make([]string, l)
	for i := 0; i < int(l); i++ {
		cfStr, _, _ := purego.SyscallN(cfArrayGetValueAtIndex, arr, uintptr(i))
		ss[i] = cfStringToGoString(CFStringRef(cfStr))
	}
	return ss
}
