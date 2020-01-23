// +build !windows

package procutil

import (
	"os"
	"os/exec"
	"syscall"
)

func SetOptNewProcessGroup(attrs *syscall.SysProcAttr) {
	attrs.Setpgid = true
}

func KillProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		// Kill the entire process group.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

func GracefullyShutdownProcess(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
