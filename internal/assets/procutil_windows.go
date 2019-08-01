// +build windows

package assets

import (
	"os/exec"
	"syscall"
)

const createNewProcessGroupFlag = 0x00000200

// https://docs.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
func setOptNewProcessGroup(attrs *syscall.SysProcAttr) {
	attrs.CreationFlags = createNewProcessGroupFlag
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}
