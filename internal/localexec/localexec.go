// Package localexec provides constructs for uniform execution of local processes,
// specifically conversion from model.Cmd to exec.Cmd.
package localexec

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Common environment for local exec commands.
type Env struct {
	pairs   []kvPair
	environ func() []string
}

func EmptyEnv() *Env {
	return &Env{
		environ: os.Environ,
	}
}

func DefaultEnv(port model.WebPort, host model.WebHost) *Env {
	e := &Env{
		environ: os.Environ,
	}

	// if Tilt was invoked with `tilt up --port=XXXXX`, local() calls to use the Tilt API will fail due to trying to
	// connect to the default port, so explicitly populate the TILT_PORT environment variable if it isn't already
	e.Add("TILT_PORT", strconv.Itoa(int(port)))

	// some Tilt commands, such as `tilt dump engine`, also require the host
	e.Add("TILT_HOST", string(host))

	// We don't really want to collect analytics when extensions use 'tilt get'/'tilt apply'.
	e.Add("TILT_DISABLE_ANALYTICS", "1")

	return e
}

func (e *Env) Add(k, v string) {
	e.pairs = append(e.pairs, kvPair{Key: k, Value: v})
}

// ExecCmd creates a stdlib exec.Cmd instance suitable for execution by the local engine.
//
// The resulting command will inherit the parent process (i.e. `tilt`) environment, then
// have command specific environment overrides applied, and finally, additional conditional
// environment to improve logging output.
//
// NOTE: To avoid confusion with ExecCmdContext, this method accepts a logger instance
// directly rather than using logger.Get(ctx); the returned exec.Cmd from this function
// will NOT be associated with any context.
func (e *Env) ExecCmd(cmd model.Cmd, l logger.Logger) (*exec.Cmd, error) {
	if len(cmd.Argv) == 0 {
		return nil, errors.New("empty cmd")
	}
	c := exec.Command(cmd.Argv[0], cmd.Argv[1:]...)
	e.populateExecCmd(c, cmd, l)
	return c, nil
}

func (e *Env) populateExecCmd(c *exec.Cmd, cmd model.Cmd, l logger.Logger) {
	c.Dir = cmd.Dir
	// env precedence: parent process (i.e. tilt) -> logger -> command
	// dupes are left for Go stdlib to handle (API guarantees last wins)
	execEnv := e.environ()

	execEnv = logger.PrepareEnv(l, execEnv)
	for _, kv := range e.pairs {
		execEnv = addEnvIfNotPresent(execEnv, kv.Key, kv.Value)
	}

	execEnv = append(execEnv, cmd.Env...)
	c.Env = execEnv
}

type kvPair struct {
	Key   string
	Value string
}

func addEnvIfNotPresent(env []string, key, value string) []string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return env
		}
	}

	return append(env, key+"="+value)
}
