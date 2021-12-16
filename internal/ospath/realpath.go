package ospath

// #include <limits.h>
// #include <stdio.h>
// #include <stdlib.h>
import "C"
import (
	"path/filepath"
	"unsafe"
)

// Evaluate a path to the proper casing, resolving all symlinks.
func Canonicalize(path string) (string, error) {
	current := path
	remainder := ""
	for {
		result, err := Realpath(current)
		if err == nil {
			return filepath.Join(result, remainder), nil
		}

		next := filepath.Dir(current)
		if next == current {
			return "", err
		}

		remainder = filepath.Join(filepath.Base(current), remainder)
		current = next
	}
}

// Go binding for realpath from glibc.
func Realpath(path string) (string, error) {
	// Allocate a C *char for the input.
	cstr := C.CString(path)
	defer C.free(unsafe.Pointer(cstr))

	// Allocate a C *char for the output.
	var outputBytes [C.PATH_MAX]byte
	outputCstr := (*C.char)(unsafe.Pointer(&outputBytes[0]))
	resultCstr, err := C.realpath(cstr, outputCstr)

	// CGo error handling doesn't work like normal Go error handling.
	// The CGo function returns errno wrapped as an error, but
	// may return an error even if the function succeeds.
	//
	// See:
	// https://pkg.go.dev/cmd/cgo#hdr-Go_references_to_C
	// https://bugzilla.redhat.com/show_bug.cgi?id=1916968
	result := C.GoString(resultCstr)
	if result == "" && err != nil {
		return "", err
	}
	return result, nil
}
