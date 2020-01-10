package local

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type Execer interface {
	Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan status) chan struct{}
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

func (e *FakeExecer) Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan status) chan struct{} {
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

func fakeRun(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan status, doneCh chan struct{}, exitCh chan int) {
	defer close(doneCh)
	defer close(statusCh)

	_, _ = fmt.Fprintf(w, "Starting cmd %v", cmd)

	statusCh <- Running

	select {
	case <-ctx.Done():
		_, _ = fmt.Fprintf(w, "cmd %v canceled", cmd)
		// this was cleaned up by the controller, so it's not an error
		statusCh <- Done
	case exitCode := <-exitCh:
		_, _ = fmt.Fprintf(w, "cmd %v exited with code %d", cmd, exitCode)
		// even an exit code of 0 is an error, because services aren't supposed to exit!
		statusCh <- Error
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

func (e *processExecer) Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan status) chan struct{} {
	doneCh := make(chan struct{})

	go processRun(ctx, cmd, w, statusCh, doneCh)

	return doneCh
}

func processRun(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan status, doneCh chan struct{}) {
	defer close(doneCh)
	defer close(statusCh)

	c := exec.CommandContext(ctx, cmd.Argv[0], cmd.Argv[1:]...)
	c.Stderr = w
	c.Stdout = w

	err := c.Start()
	if err != nil {
		// TODO(dmiller): should this be different status than when the command fails? Unknown?
		statusCh <- Error
		return
	}

	statusCh <- Running

	err = c.Wait()
	if err != nil {
		// TODO(matt) don't log error if it was killed by tilt
		// TODO(matt) how do we get this to show up as an error in the web UI?
		_, err = fmt.Fprintf(w, "Error execing %s: %v", cmd.String(), err)
		if err != nil {
			logger.Get(ctx).Infof("Unable to print exec output to writer: %v", err)
		}
		statusCh <- Error
		return
	}

	// TODO(matt) if the process exits w/ exit code 0, Tilt shows it as green - should it be red?
	statusCh <- Done
}
