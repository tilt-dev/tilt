// +build windows

package procutil

import (
	"os/exec"
	"syscall"
)

// https://docs.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
func setOptNewProcessGroup(attrs *syscall.SysProcAttr) {
	attrs.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}
