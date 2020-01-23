// +build windows

package procutil

import (
	"context"
	"os"
	"os/exec"
	"syscall"

	"github.com/gentlemanautomaton/graceful"
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

func GracefullyShutdownProcess(p *os.Process) error {
	return graceful.Exit(context.Background(), p.Pid, 1)
}
