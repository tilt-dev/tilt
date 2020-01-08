package local

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/windmilleng/tilt/pkg/model"
)

type Execer interface {
	Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan Status) chan struct{}
}

type FakeExecer struct{}

func NewFakeExecer() *FakeExecer {
	return &FakeExecer{}
}

func (e *FakeExecer) Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan Status) chan struct{} {
	doneCh := make(chan struct{})
	go fakeRun(ctx, cmd, w, statusCh, doneCh)

	return doneCh
}

func fakeRun(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan Status, doneCh chan struct{}) {
	defer close(doneCh)
	defer close(statusCh)

	_, _ = fmt.Fprintf(w, "Starting cmd %v", cmd)

	statusCh <- Running

	<-ctx.Done()
	_, _ = fmt.Fprintf(w, "Finished cmd %v", cmd)

	statusCh <- Done
}

type processExecer struct{}

func NewProcessExecer() *processExecer {
	return &processExecer{}
}

func (e *processExecer) Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan Status) chan struct{} {
	doneCh := make(chan struct{})

	go processRun(ctx, cmd, w, statusCh, doneCh)

	return doneCh
}

func processRun(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan Status, doneCh chan struct{}) {
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
		statusCh <- Error
		return
	}

	statusCh <- Done
}
