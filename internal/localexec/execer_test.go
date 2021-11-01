package localexec

import (
	"context"
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
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestProcessExecer_Run(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	// this works across both sh + cmd
	script := `echo hello from stdout && echo hello from stderr 1>&2`

	execer := NewProcessExecer(EmptyEnv())

	r, err := OneShot(ctx, execer, model.ToHostCmd(script))

	require.NoError(t, err)
	assert.Equal(t, 0, r.ExitCode)
	assert.Equal(t, "hello from stdout\n", string(r.Stdout))
	assert.Equal(t, "hello from stderr\n", string(r.Stderr))
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

	execer := NewProcessExecer(EmptyEnv())
	r, err := OneShot(ctx, execer, model.ToUnixCmd(script))

	require.NoError(t, err)
	assert.Equal(t, 137, r.ExitCode)

	output := strings.TrimSpace(string(r.Stdout))
	if assert.NotEmpty(t, output) {
		childPid, err := strconv.Atoi(output)
		require.NoError(t, err, "Couldn't get child PID from stdout/stderr: %s", output)
		// os.FindProcess is a no-op on Unix-like systems and always succeeds; need to send signal 0 to probe it
		proc, _ := os.FindProcess(childPid)
		err = proc.Signal(syscall.Signal(0))
		if !assert.Equal(t, os.ErrProcessDone, err, "Child process was still running") {
			_ = proc.Kill()
		}
	}
}
