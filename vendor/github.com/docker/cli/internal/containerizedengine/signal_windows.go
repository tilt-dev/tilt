// +build windows

package containerizedengine

import (
	"syscall"
)

var (
	// SIGKILL all signals are ignored by containerd kill windows
	SIGKILL = syscall.Signal(0)
)
