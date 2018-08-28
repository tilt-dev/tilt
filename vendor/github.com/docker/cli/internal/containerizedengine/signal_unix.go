// +build !windows

package containerizedengine

import (
	"golang.org/x/sys/unix"
)

var (
	// SIGKILL maps to unix.SIGKILL
	SIGKILL = unix.SIGKILL
)
