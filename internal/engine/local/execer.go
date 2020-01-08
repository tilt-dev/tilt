package local

import (
	"context"
	"fmt"
	"io"

	"github.com/windmilleng/tilt/pkg/model"
)

type Execer interface {
	Start(ctx context.Context, cmd model.Cmd, w io.Writer, statusCh chan Status) chan struct{}
}

type FakeExecer struct {
}

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
