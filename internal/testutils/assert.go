package testutils

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func IsNotExistMessage() string {
	if runtime.GOOS == "windows" {
		return "The system cannot find" // some error messages use "path" and some use "file"
	} else {
		return "no such file or directory"
	}

}

// Asserts the given error indicates a file doesn't exist.
// Uses string matching instead of type-checking, to workaround
// libraries that wrap the error.
func AssertIsNotExist(t *testing.T, err error) {
	assert.Contains(t, err.Error(), IsNotExistMessage())
}
