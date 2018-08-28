// +build !windows

package containerized

import (
	"golang.org/x/sys/unix"
)

var (
	// sigTERM maps to unix.SIGTERM
	sigTERM = unix.SIGTERM
)
