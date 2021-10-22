// +build windows

package procutil

import (
	"fmt"
	"os/exec"
	"syscall"
)

func SetOptNewProcessGroup(attrs *syscall.SysProcAttr) {
}

func KillProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = exec.Command("TASKKILL", "/T", "/F", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
	}
}
