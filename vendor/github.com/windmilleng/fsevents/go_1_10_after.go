// +build darwin,go1.10

package fsevents

/*
#include <CoreServices/CoreServices.h>
*/
import "C"
import "unsafe"

const (
	nullCFAllocatorRef = C.CFAllocatorRef(0)
	nullCFStringRef    = C.CFStringRef(0)
	nullCFUUIDRef      = C.CFUUIDRef(0)
)

// NOTE: The following code is identical between go_1_10_after and go_1_10_before,
// however versions of Go 1.10.x prior to 1.10.4 fail to compile when the code utilizing
// the above constants is in a different file (wrap.go).

// GetDeviceUUID retrieves the UUID required to identify an EventID
// in the FSEvents database
func GetDeviceUUID(deviceID int32) string {
	uuid := C.FSEventsCopyUUIDForDevice(C.dev_t(deviceID))
	if uuid == nullCFUUIDRef {
		return ""
	}
	return cfStringToGoString(C.CFUUIDCreateString(nullCFAllocatorRef, uuid))
}

// makeCFString makes an immutable string with CFStringCreateWithCString.
func makeCFString(str string) C.CFStringRef {
	s := C.CString(str)
	defer C.free(unsafe.Pointer(s))
	return C.CFStringCreateWithCString(nullCFAllocatorRef, s, C.kCFStringEncodingUTF8)
}

// copyCFString makes an immutable copy of a string with CFStringCreateCopy.
func copyCFString(cfs C.CFStringRef) C.CFStringRef {
	return C.CFStringCreateCopy(nullCFAllocatorRef, cfs)
}
