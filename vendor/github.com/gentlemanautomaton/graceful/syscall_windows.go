// +build windows

package graceful

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Event constants.
const (
	infinite   = 0xffffffff
	waitObject = 0 // WAIT_OBJECT_0
)

var (
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procCreateEventW           = modkernel32.NewProc("CreateEventW")
	procSetEvent               = modkernel32.NewProc("SetEvent")
	procWaitForMultipleObjects = modkernel32.NewProc("WaitForMultipleObjects")
	procOpenProcess            = modkernel32.NewProc("OpenProcess")
	procExitProcess            = modkernel32.NewProc("ExitProcess")
	procCreateRemoteThread     = modkernel32.NewProc("CreateRemoteThread")
	procIsWow64Process         = modkernel32.NewProc("IsWow64Process")
)

func createEvent(sa *syscall.SecurityAttributes, manualReset, initialState bool, name string) (event syscall.Handle, err error) {
	var pname *uint16
	pname, err = utf16PtrFromString(name)
	if err != nil {
		return
	}
	r0, _, e1 := syscall.Syscall6(
		procCreateEventW.Addr(),
		4,
		uintptr(unsafe.Pointer(sa)),
		boolToUintptr(manualReset),
		boolToUintptr(initialState),
		uintptr(unsafe.Pointer(pname)),
		0,
		0)
	if r0 == 0 {
		event = syscall.InvalidHandle
		if e1 != 0 {
			err = os.NewSyscallError("CreateEvent", e1)
		} else {
			err = syscall.EINVAL
		}
	} else {
		event = syscall.Handle(r0)
	}
	return
}

func setEvent(event syscall.Handle) (err error) {
	r0, _, e1 := syscall.Syscall(
		procSetEvent.Addr(),
		1,
		uintptr(event),
		0,
		0)
	if r0 == 0 {
		if e1 != 0 {
			err = os.NewSyscallError("SetEvent", e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func waitForMultipleObjects(objects []syscall.Handle, waitAll bool, milliseconds uint32) (event uint32, err error) {
	if len(objects) == 0 {
		return 0, os.NewSyscallError("WaitForMultipleObjects", errors.New("object count cannot be zero"))
	}
	r0, _, e1 := syscall.Syscall6(
		procWaitForMultipleObjects.Addr(),
		4,
		uintptr(len(objects)),
		uintptr(unsafe.Pointer(&objects[0])),
		boolToUintptr(waitAll),
		uintptr(milliseconds),
		0,
		0)
	event = uint32(r0)
	if r0 == 0xffffffff {
		if e1 != 0 {
			err = os.NewSyscallError("WaitForMultipleObjects", e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func openProcess(desiredAccess uint32, inheritHandle bool, processID uint32) (process syscall.Handle, err error) {
	var ih uintptr
	if inheritHandle {
		ih = 1
	}
	r0, _, e1 := syscall.Syscall(
		procOpenProcess.Addr(),
		3,
		uintptr(desiredAccess),
		uintptr(ih),
		uintptr(processID))
	if r0 == 0 {
		process = syscall.InvalidHandle
		if e1 != 0 {
			err = os.NewSyscallError("OpenProcess", e1)
		} else {
			err = syscall.EINVAL
		}
	} else {
		process = syscall.Handle(r0)
	}
	return
}

func createRemoteThread(process syscall.Handle, sa *syscall.SecurityAttributes, stackSize uint32, startAddress, parameter uintptr, creationFlags uint32) (thread syscall.Handle, id uint32, err error) {
	r0, _, e1 := syscall.Syscall9(
		procCreateRemoteThread.Addr(),
		7,
		uintptr(process),
		uintptr(unsafe.Pointer(sa)),
		uintptr(stackSize),
		startAddress,
		parameter,
		uintptr(creationFlags),
		uintptr(unsafe.Pointer(&id)), 0, 0)
	if r0 == 0 {
		thread = syscall.InvalidHandle
		if e1 != 0 {
			err = os.NewSyscallError("CreateRemoteThread", e1)
		} else {
			err = syscall.EINVAL
		}
	} else {
		thread = syscall.Handle(r0)
	}
	return
}

func isWow64Process(process syscall.Handle) (result bool, err error) {
	var wow64 uintptr
	r0, _, e1 := syscall.Syscall(
		procIsWow64Process.Addr(),
		2,
		uintptr(process),
		uintptr(unsafe.Pointer(&wow64)),
		0)
	if r0 == 0 {
		if e1 != 0 {
			err = os.NewSyscallError("IsWow64Process", e1)
		} else {
			err = syscall.EINVAL
		}
	} else if wow64 != 0 {
		result = true
	}
	return
}

func utf16PtrFromString(value string) (pvalue *uint16, err error) {
	if value != "" {
		pvalue, err = syscall.UTF16PtrFromString(value)
	}
	return
}

func boolToUintptr(value bool) uintptr {
	if value {
		return 1
	}
	return 0
}
