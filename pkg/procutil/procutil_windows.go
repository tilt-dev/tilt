// +build windows

package procutil

import (
	"os/exec"
	"syscall"
)

// https://docs.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
func SetOptNewProcessGroup(attrs *syscall.SysProcAttr) {
	attrs.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
}

func KillProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}
