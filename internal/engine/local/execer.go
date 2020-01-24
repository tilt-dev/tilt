package local

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

	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/procutil"
)

type Execer interface {
	Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan statusAndMetadata, spanID model.LogSpanID) chan struct{}
}

type fakeExecProcess struct {
	exitCh chan int
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
		exitCh: exitCh,
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

	_, _ = fmt.Fprintf(w, "Starting cmd %v", cmd)

	statusCh <- statusAndMetadata{status: Running}

	select {
	case <-ctx.Done():
		_, _ = fmt.Fprintf(w, "cmd %v canceled", cmd)
		// this was cleaned up by the controller, so it's not an error
		statusCh <- statusAndMetadata{status: Done}
	case exitCode := <-exitCh:
		_, _ = fmt.Fprintf(w, "cmd %v exited with code %d", cmd, exitCode)
		// even an exit code of 0 is an error, because services aren't supposed to exit!
		statusCh <- statusAndMetadata{status: Error}
	}
}

func (fe *FakeExecer) RequireNoKnownProcess(t *testing.T, cmd string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	_, ok := fe.processes[cmd]

	require.False(t, ok, "%T should not be tracking any process with cmd %q, but it is", FakeExecer{}, cmd)
}

func ProvideExecer() Execer {
	return NewProcessExecer()
}

type processExecer struct{}

func NewProcessExecer() *processExecer {
	return &processExecer{}
}

func (e *processExecer) Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan statusAndMetadata, spanID model.LogSpanID) chan struct{} {
	doneCh := make(chan struct{})

	go processRun(ctx, cmd, w, statusCh, doneCh, spanID)

	return doneCh
}

func processRun(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan statusAndMetadata, doneCh chan struct{}, spanID model.LogSpanID) {
	defer close(doneCh)
	defer close(statusCh)

	logger.Get(ctx).Infof("Running serve cmd: %s", cmd.String())
	c := exec.Command(cmd.Argv[0], cmd.Argv[1:]...)

	c.SysProcAttr = &syscall.SysProcAttr{}
	procutil.SetOptNewProcessGroup(c.SysProcAttr)
	c.Stderr = w
	c.Stdout = w

	err := c.Start()
	if err != nil {
		logger.Get(ctx).Errorf("%s failed to start: %v", cmd.String(), err)
		statusCh <- statusAndMetadata{status: Error, spanID: spanID}
		return
	}

	statusCh <- statusAndMetadata{status: Running, pid: c.Process.Pid, spanID: spanID}

	// This is to prevent this goroutine from blocking, since we know there's only going to be one result
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Wait()
	}()

	select {
	case err := <-errCh:
		if err == nil {
			logger.Get(ctx).Errorf("%s exited with exit code 0", cmd.String())
		} else if ee, ok := err.(*exec.ExitError); ok {
			logger.Get(ctx).Errorf("%s exited with exit code %d", cmd.String(), ee.ExitCode())
		} else {
			logger.Get(ctx).Errorf("error execing %s: %v", cmd.String(), err)
		}
		statusCh <- statusAndMetadata{status: Error, spanID: spanID}
	case <-ctx.Done():
		err := procutil.GracefullyShutdownProcess(c.Process)
		if err != nil {
			procutil.KillProcessGroup(c)
		} else {
			// wait and then send SIGKILL to the process group, unless the command finished
			select {
			case <-time.After(50 * time.Millisecond):
				procutil.KillProcessGroup(c)
			case <-doneCh:
			}
		}
		statusCh <- statusAndMetadata{status: Done, spanID: spanID}
	}
}
