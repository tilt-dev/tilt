package localexec

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestProcessExecer_Run(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	// this works across both cmd.exe + sh
	script := `echo hello from stdout && echo hello from stderr 1>&2`

	execer := NewProcessExecer(EmptyEnv())

	r, err := OneShot(ctx, execer, model.ToHostCmd(script))

	require.NoError(t, err)
	assert.Equal(t, 0, r.ExitCode)
	// trim space to not deal with line-ending/whitespace differences between cmd.exe/sh
	assert.Equal(t, "hello from stdout", strings.TrimSpace(string(r.Stdout)))
	assert.Equal(t, "hello from stderr", strings.TrimSpace(string(r.Stderr)))
}

func TestProcessExecer_Run_ProcessGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("test not supported on Windows")
	}

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	script := `sleep 60 & echo $!`

	// to speed up test execution, as soon as we see the PID written to stdout, cancel the context
	// to trigger process termination
	var childPid int
	stdoutBuf := bufsync.NewThreadSafeBuffer()
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			output := strings.TrimSpace(stdoutBuf.String())
			if output != "" {
				var err error
				childPid, err = strconv.Atoi(output)
				if err == nil {
					cancel()
					return
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	execer := NewProcessExecer(EmptyEnv())
	exitCode, err := execer.Run(ctx, model.ToUnixCmd(script), RunIO{Stdout: stdoutBuf})

	require.NoError(t, err)
	assert.Equal(t, 137, exitCode)

	if assert.NotZero(t, childPid, "Process did not write child PID to stdout") {
		// os.FindProcess is a no-op on Unix-like systems and always succeeds; need to send signal 0 to probe it
		proc, _ := os.FindProcess(childPid)
		childProcStopped := assert.Eventually(t, func() bool {
			err = proc.Signal(syscall.Signal(0))
			return errors.Is(err, os.ErrProcessDone)
		}, time.Second, 50*time.Millisecond, "Child process was still running")
		if !childProcStopped {
			_ = proc.Kill()
		}
	}
}
