// +build !windows

package assets

import (
	"os/exec"
	"syscall"
)

func setOptNewProcessGroup(attrs *syscall.SysProcAttr) {
	attrs.Setpgid = true
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		// Kill the entire process group.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
