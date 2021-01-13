package localexec

import (
	"context"
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

// ExecCmdContext is like ExecCmd but uses exec.CommandContext to associate a context with
// the returned exec.Cmd.
func ExecCmdContext(ctx context.Context, cmd model.Cmd) *exec.Cmd {
	c := exec.CommandContext(ctx, cmd.Argv[0], cmd.Argv[1:]...)
	populateExecCmd(c, cmd, logger.Get(ctx))
	return c
}

func populateExecCmd(c *exec.Cmd, cmd model.Cmd, l logger.Logger) {
	c.Dir = cmd.Dir
	// env from command definition takes precedence over parent env (exec.Cmd takes last in case of dupes)
	execEnv := os.Environ()
	execEnv = append(execEnv, cmd.Env...)
	c.Env = logger.PrepareEnv(l, execEnv)
}
