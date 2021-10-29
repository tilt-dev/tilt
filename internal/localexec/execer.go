package localexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/procutil"
)

// OneShotResult includes details about command execution.
type OneShotResult struct {
	// ExitCode from the process
	ExitCode int
	// Stdout from the process
	Stdout []byte
	// Stderr from the process
	Stderr []byte
}

type RunIO struct {
	// Stdin for the process
	Stdin io.Reader
	// Stdout for the process
	Stdout io.Writer
	// Stderr for the process
	Stderr io.Writer
}

type Execer interface {
	// Run executes a command and waits for it to complete.
	//
	// If the context is canceled before the process terminates, the process will be killed.
	Run(ctx context.Context, cmd model.Cmd, runIO RunIO) (int, error)
}

func OneShot(ctx context.Context, execer Execer, cmd model.Cmd) (OneShotResult, error) {
	var stdout, stderr bytes.Buffer
	runIO := RunIO{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	exitCode, err := execer.Run(ctx, cmd, runIO)
	if err != nil {
		return OneShotResult{}, err
	}

	return OneShotResult{
		ExitCode: exitCode,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
}

type ProcessExecer struct {
	env *Env
}

var _ Execer = &ProcessExecer{}

func NewProcessExecer(env *Env) *ProcessExecer {
	return &ProcessExecer{env: env}
}

func (p ProcessExecer) Run(ctx context.Context, cmd model.Cmd, runIO RunIO) (int, error) {
	osCmd, err := p.env.ExecCmd(cmd, logger.Get(ctx))
	if err != nil {
		return -1, err
	}

	osCmd.SysProcAttr = &syscall.SysProcAttr{}
	procutil.SetOptNewProcessGroup(osCmd.SysProcAttr)

	osCmd.Stdin = runIO.Stdin
	osCmd.Stdout = runIO.Stdout
	osCmd.Stderr = runIO.Stderr

	if err := osCmd.Start(); err != nil {
		return -1, err
	}

	// there's a data race on this value if the process termination + context cancellation happen
	// very close together, so all access should be done via atomic
	//
	// NOTE: if this condition occurs, it's possible the process will be reported as killed (127)
	// even though it actually had already exited because we force an exit code to prevent returning
	// an exit code of 0 despite having triggered a SIGKILL in the common case
	var exitCode int64
	procDone := make(chan struct{}, 1)
	go func() {
		select {
		case <-procDone:
			// stop blocking & do nothing, process already terminated
			return
		case <-ctx.Done():
			// forcibly set the exit code to simulate a standard SIGKILL
			// (since we're signaling the process group, it's possible to still get an exit code of 0 otherwise)
			atomic.StoreInt64(&exitCode, 137)
			procutil.KillProcessGroup(osCmd)
		}
	}()

	// this WILL block on child processes, but that's ok since we handle the timeout termination in a goroutine above
	// and it's preferable vs using Process::Wait() since that complicates I/O handling (Cmd::Wait() will
	// ensure all I/O is complete before returning)
	err = osCmd.Wait()
	procDone <- struct{}{}
	close(procDone)
	if exitErr, ok := err.(*exec.ExitError); ok {
		atomic.StoreInt64(&exitCode, int64(exitErr.ExitCode()))
		err = nil
	} else if err != nil {
		exitCode = -1
	}

	// this conversion should be safe:
	// 	* Windows exit codes are 32-bit
	// 	* Unix exit codes are platform int
	//		^ the only value we explicitly store is 127 so in range even if 32-bit
	return int(atomic.LoadInt64(&exitCode)), err
}

type fakeCmdResult struct {
	exitCode int
	err      error
	stdout   string
	stderr   string
}

type FakeExecer struct {
	t  testing.TB
	mu sync.Mutex

	cmds map[string]fakeCmdResult
}

var _ Execer = &FakeExecer{}

func NewFakeExecer(t testing.TB) *FakeExecer {
	return &FakeExecer{
		t: t,
	}
}

func (f *FakeExecer) Run(ctx context.Context, cmd model.Cmd, runIO RunIO) (int, error) {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()

	ctxErr := ctx.Err()
	if ctxErr != nil {
		return -1, ctxErr
	}

	if r, ok := f.cmds[cmd.String()]; ok {
		if r.err != nil {
			return -1, r.err
		}

		if _, err := runIO.Stdout.Write([]byte(r.stdout)); err != nil {
			return -1, fmt.Errorf("error writing to stdout: %v", err)
		}

		if _, err := runIO.Stderr.Write([]byte(r.stderr)); err != nil {
			return -1, fmt.Errorf("error writing to stderr: %v", err)
		}

		return r.exitCode, nil
	}

	return 0, nil
}

func (f *FakeExecer) RegisterCommandError(cmd string, err error) {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cmds[cmd] = fakeCmdResult{
		err: err,
	}
}

func (f *FakeExecer) RegisterCommand(cmd string, exitCode int, stdout string, stderr string) {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cmds[cmd] = fakeCmdResult{
		exitCode: exitCode,
		stdout:   stdout,
		stderr:   stderr,
	}
}
