package localexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
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

func OneShotToLogger(ctx context.Context, execer Execer, cmd model.Cmd) error {
	l := logger.Get(ctx)
	out := l.Writer(logger.InfoLvl)

	runIO := RunIO{Stdout: out, Stderr: out}

	l.Infof("Running cmd: %s", cmd.String())
	exitCode, err := execer.Run(ctx, cmd, runIO)
	if err == nil && exitCode != 0 {
		err = fmt.Errorf("exit status %d", exitCode)
	}
	return err
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

	// monitor context cancel in a background goroutine and forcibly kill the process group if it's exceeded
	// (N.B. an exit code of 137 is forced; otherwise, it's possible for the main process to exit with 0 after
	// its children are killed, which is misleading)
	// the sync.Once provides synchronization with the main function that's blocked on Cmd::Wait()
	var exitCode int
	var handleProcessExit sync.Once
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		handleProcessExit.Do(
			func() {
				procutil.KillProcessGroup(osCmd)
				exitCode = 137
			})
	}()

	// this WILL block on child processes, but that's ok since we handle the timeout termination in a goroutine above
	// and it's preferable vs using Process::Wait() since that complicates I/O handling (Cmd::Wait() will
	// ensure all I/O is complete before returning)
	err = osCmd.Wait()
	if exitErr, ok := err.(*exec.ExitError); ok {
		handleProcessExit.Do(
			func() {
				exitCode = exitErr.ExitCode()
			})
		err = nil
	} else if err != nil {
		handleProcessExit.Do(
			func() {
				exitCode = -1
			})
	} else {
		// explicitly consume the sync.Once to prevent a data race with the goroutine waiting on the context
		// (since process completed successfully, exit code is 0, so no need to set anything)
		handleProcessExit.Do(func() {})
	}
	return exitCode, err
}

type fakeCmdResult struct {
	exitCode int
	err      error
	stdout   []byte
	stderr   []byte
}

type FakeCall struct {
	Cmd      model.Cmd
	ExitCode int
	Error    error
}

func (f FakeCall) String() string {
	return fmt.Sprintf("cmd=%q exitCode=%d err=%v", f.Cmd.String(), f.ExitCode, f.Error)
}

type FakeExecer struct {
	t  testing.TB
	mu sync.Mutex

	cmds map[string]fakeCmdResult

	calls []FakeCall
}

var _ Execer = &FakeExecer{}

func NewFakeExecer(t testing.TB) *FakeExecer {
	return &FakeExecer{
		t:    t,
		cmds: make(map[string]fakeCmdResult),
	}
}

func (f *FakeExecer) Run(ctx context.Context, cmd model.Cmd, runIO RunIO) (exitCode int, err error) {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()

	defer func() {
		f.calls = append(f.calls, FakeCall{
			Cmd:      cmd,
			ExitCode: exitCode,
			Error:    err,
		})
	}()

	ctxErr := ctx.Err()
	if ctxErr != nil {
		return -1, ctxErr
	}

	if r, ok := f.cmds[cmd.String()]; ok {
		if r.err != nil {
			return -1, r.err
		}

		if runIO.Stdout != nil && len(r.stdout) != 0 {
			if _, err := runIO.Stdout.Write(r.stdout); err != nil {
				return -1, fmt.Errorf("error writing to stdout: %v", err)
			}
		}

		if runIO.Stderr != nil && len(r.stderr) != 0 {
			if _, err := runIO.Stderr.Write(r.stderr); err != nil {
				return -1, fmt.Errorf("error writing to stderr: %v", err)
			}
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

// RegisterCommandBytes adds or replaces a command to the FakeExecer.
//
// The output values will be used exactly as-is, so can be used to simulate processes that do not newline terminate etc.
func (f *FakeExecer) RegisterCommandBytes(cmd string, exitCode int, stdout []byte, stderr []byte) {
	f.registerCommand(cmd, exitCode, stdout, stderr)
}

// RegisterCommand adds or replaces a command to the FakeExecer.
//
// If the output strings are not newline terminated, a newline will automatically be added.
// If this behavior is not desired, use `RegisterCommandBytes`.
func (f *FakeExecer) RegisterCommand(cmd string, exitCode int, stdout string, stderr string) {
	if stdout != "" && !strings.HasSuffix(stdout, "\n") {
		stdout += "\n"
	}

	if stderr != "" && !strings.HasSuffix(stderr, "\n") {
		stderr += "\n"
	}

	f.registerCommand(cmd, exitCode, []byte(stdout), []byte(stderr))
}

func (f *FakeExecer) Calls() []FakeCall {
	return f.calls
}

func (f *FakeExecer) registerCommand(cmd string, exitCode int, stdout []byte, stderr []byte) {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cmds[cmd] = fakeCmdResult{
		exitCode: exitCode,
		stdout:   stdout,
		stderr:   stderr,
	}
}
