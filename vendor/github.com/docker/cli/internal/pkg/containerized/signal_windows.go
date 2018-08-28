// +build windows

package containerized

import (
	"syscall"
)

var (
	// sigTERM all signals are ignored by containerd kill windows
	sigTERM = syscall.Signal(0)
)
