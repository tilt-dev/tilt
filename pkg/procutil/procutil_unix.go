// +build !windows

package procutil

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
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
	err := syscall.Kill(-p.Pid, syscall.SIGTERM)
	if err != nil {
		return errors.Wrap(err, "SIGTERM")
	}

	err = syscall.Kill(-p.Pid, syscall.SIGINT)
	return errors.Wrap(err, "SIGINT")
}
