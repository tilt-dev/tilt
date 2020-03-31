package testutils

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Asserts the given error indicates a file doesn't exist.
// Uses string matching instead of type-checking, to workaround
// libraries that wrap the error.
func AssertIsNotExist(t *testing.T, err error) {
	if runtime.GOOS == "windows" {
		assert.Contains(t, err.Error(), "The system cannot find the file specified")
	} else {
		assert.Contains(t, err.Error(), "no such file or directory")
	}
}
