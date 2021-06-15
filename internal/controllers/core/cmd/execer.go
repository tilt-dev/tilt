package cmd

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/procutil"
)

var DefaultGracePeriod = 30 * time.Second

type Execer interface {
	Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan statusAndMetadata, spanID model.LogSpanID) chan struct{}
}

type fakeExecProcess struct {
	exitCh    chan int
	workdir   string
	startTime time.Time
}

type FakeExecer struct {
	// really dumb/simple process management - key by the command string, and make duplicates an error
	processes map[string]*fakeExecProcess
	mu        sync.Mutex
}

func NewFakeExecer() *FakeExecer {
	return &FakeExecer{
		processes: make(map[string]*fakeExecProcess),
	}
}

func (e *FakeExecer) Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan statusAndMetadata, spanID model.LogSpanID) chan struct{} {
	e.mu.Lock()
	_, ok := e.processes[cmd.String()]
	e.mu.Unlock()
	if ok {
		logger.Get(ctx).Infof("internal error: fake execer only supports one instance of each unique command at a time. tried to start a second instance of %q", cmd.Argv)
		return nil
	}

	exitCh := make(chan int)

	e.mu.Lock()
	e.processes[cmd.String()] = &fakeExecProcess{
		exitCh:    exitCh,
		workdir:   cmd.Dir,
		startTime: time.Now(),
	}
	e.mu.Unlock()

	doneCh := make(chan struct{})
	go func() {
		fakeRun(ctx, cmd, w, statusCh, doneCh, exitCh)

		e.mu.Lock()
		delete(e.processes, cmd.String())
		e.mu.Unlock()
	}()

	return doneCh
}

// stops the command with the given command, faking the specified exit code
func (e *FakeExecer) stop(cmd string, exitCode int) error {
	e.mu.Lock()
	p, ok := e.processes[cmd]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("no such process %q", cmd)
	}

	p.exitCh <- exitCode
	e.mu.Lock()
	delete(e.processes, cmd)
	e.mu.Unlock()
	return nil
}

func fakeRun(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan statusAndMetadata, doneCh chan struct{}, exitCh chan int) {
	defer close(doneCh)
	defer close(statusCh)

	_, _ = fmt.Fprintf(w, "Starting cmd %v\n", cmd)

	statusCh <- statusAndMetadata{status: Running}

	select {
	case <-ctx.Done():
		_, _ = fmt.Fprintf(w, "cmd %v canceled\n", cmd)
		// this was cleaned up by the controller, so it's not an error
		statusCh <- statusAndMetadata{status: Done, exitCode: 0}
	case exitCode := <-exitCh:
		_, _ = fmt.Fprintf(w, "cmd %v exited with code %d\n", cmd, exitCode)
		// even an exit code of 0 is an error, because services aren't supposed to exit!
		statusCh <- statusAndMetadata{status: Error, exitCode: exitCode}
	}
}

func (fe *FakeExecer) RequireNoKnownProcess(t *testing.T, cmd string) {
	t.Helper()
	fe.mu.Lock()
	defer fe.mu.Unlock()

	_, ok := fe.processes[cmd]

	require.False(t, ok, "%T should not be tracking any process with cmd %q, but it is", FakeExecer{}, cmd)
}

func ProvideExecer() Execer {
	return NewProcessExecer()
}

type processExecer struct {
	gracePeriod time.Duration
}

func NewProcessExecer() *processExecer {
	return &processExecer{
		gracePeriod: DefaultGracePeriod,
	}
}

func (e *processExecer) Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan statusAndMetadata, spanID model.LogSpanID) chan struct{} {
	doneCh := make(chan struct{})

	go func() {
		e.processRun(ctx, cmd, w, statusCh, spanID)
		close(doneCh)
	}()

	return doneCh
}

func (e *processExecer) processRun(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan statusAndMetadata, spanID model.LogSpanID) {
	defer close(statusCh)

	logger.Get(ctx).Infof("Running cmd: %s", cmd.String())
	c := localexec.ExecCmd(cmd, logger.Get(ctx))

	c.SysProcAttr = &syscall.SysProcAttr{}
	procutil.SetOptNewProcessGroup(c.SysProcAttr)
	c.Stderr = w
	c.Stdout = w

	err := c.Start()
	if err != nil {
		logger.Get(ctx).Errorf("%s failed to start: %v", cmd.String(), err)
		statusCh <- statusAndMetadata{
			status:   Error,
			spanID:   spanID,
			exitCode: 1,
			reason:   fmt.Sprintf("failed to start: %v", err),
		}
		return
	}

	pid := c.Process.Pid
	statusCh <- statusAndMetadata{status: Running, pid: pid, spanID: spanID}

	// This is to prevent this goroutine from blocking, since we know there's only going to be one result
	processExitCh := make(chan error, 1)
	go func() {
		// Cmd Wait() does not have quite the semantics we want,
		// because it will block indefinitely on any descendant processes.
		// This can lead to Cmd appearing to hang.
		//
		// Instead, we exit immediately if the main process exits.
		//
		// Details:
		// https://github.com/tilt-dev/tilt/issues/4456
		state, err := c.Process.Wait()
		procutil.KillProcessGroup(c)

		if err != nil {
			processExitCh <- err
		} else if !state.Success() {
			processExitCh <- &exec.ExitError{ProcessState: state}
		} else {
			processExitCh <- nil
		}
		close(processExitCh)
	}()

	select {
	case err := <-processExitCh:
		exitCode := 0
		reason := ""
		status := Done
		if err == nil {
			// Use defaults
		} else if ee, ok := err.(*exec.ExitError); ok {
			status = Error
			exitCode = ee.ExitCode()
			reason = err.Error()
			logger.Get(ctx).Errorf("%s exited with exit code %d", cmd.String(), ee.ExitCode())
		} else {
			status = Error
			exitCode = 1
			reason = err.Error()
			logger.Get(ctx).Errorf("error execing %s: %v", cmd.String(), err)
		}
		statusCh <- statusAndMetadata{status: status, pid: pid, spanID: spanID, exitCode: exitCode, reason: reason}
	case <-ctx.Done():
		e.killProcess(ctx, c, processExitCh)
		statusCh <- statusAndMetadata{status: Done, pid: pid, spanID: spanID, reason: "killed", exitCode: 137}
	}
}

func (e *processExecer) killProcess(ctx context.Context, c *exec.Cmd, processExitCh chan error) {
	logger.Get(ctx).Debugf("About to gracefully shut down process %d", c.Process.Pid)
	err := procutil.GracefullyShutdownProcess(c.Process)
	if err != nil {
		logger.Get(ctx).Debugf("Unable to gracefully kill process %d, sending SIGKILL to the process group: %v", c.Process.Pid, err)
		procutil.KillProcessGroup(c)
		return
	}

	// we wait 30 seconds to give the process enough time to finish doing any cleanup.
	// this is the same timeout that Kubernetes uses
	// TODO(dmiller): make this configurable via the Tiltfile
	infoCh := time.After(e.gracePeriod / 20)
	moreInfoCh := time.After(e.gracePeriod / 3)
	finalCh := time.After(e.gracePeriod)

	select {
	case <-infoCh:
		logger.Get(ctx).Infof("Waiting %s for process to exit... (pid: %d)", e.gracePeriod, c.Process.Pid)
	case <-processExitCh:
		return
	}

	select {
	case <-moreInfoCh:
		logger.Get(ctx).Infof("Still waiting on exit... (pid: %d)", c.Process.Pid)
	case <-processExitCh:
		return
	}

	select {
	case <-finalCh:
		logger.Get(ctx).Infof("Time is up! Sending %d a kill signal", c.Process.Pid)
		procutil.KillProcessGroup(c)
	case <-processExitCh:
		return
	}
}
