// Package localexec provides constructs for uniform execution of local processes,
// specifically conversion from model.Cmd to exec.Cmd.
package localexec

import (
	"os"
	"os/exec"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// ExecCmd creates a stdlib exec.Cmd instance suitable for execution by the local engine.
//
// The resulting command will inherit the parent process (i.e. `tilt`) environment, then
// have command specific environment overrides applied, and finally, additional conditional
// environment to improve logging output.
//
// NOTE: To avoid confusion with ExecCmdContext, this method accepts a logger instance
// directly rather than using logger.Get(ctx); the returned exec.Cmd from this function
// will NOT be associated with any context.
func ExecCmd(cmd model.Cmd, l logger.Logger) *exec.Cmd {
	c := exec.Command(cmd.Argv[0], cmd.Argv[1:]...)
	populateExecCmd(c, cmd, l)
	return c
}

func populateExecCmd(c *exec.Cmd, cmd model.Cmd, l logger.Logger) {
	c.Dir = cmd.Dir
	// env precedence: parent process (i.e. tilt) -> logger -> command
	// dupes are left for Go stdlib to handle (API guarantees last wins)
	execEnv := os.Environ()
	execEnv = logger.PrepareEnv(l, execEnv)
	execEnv = append(execEnv, cmd.Env...)
	c.Env = execEnv
}
