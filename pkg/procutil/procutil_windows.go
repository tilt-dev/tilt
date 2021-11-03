// +build windows

package procutil

import (
	"fmt"
	"os"
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

func GracefullyShutdownProcess(p *os.Process) error {
	return exec.Command("TASKKILL", "/T", "/PID", fmt.Sprintf("%d", p.Pid)).Run()
}
