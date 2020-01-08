package local

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/pkg/model"
)

func TestTrue(t *testing.T) {
	f := newProcessExecFixture(t)
	f.start("true")

	f.assertCmdSucceeds()
}

func TestSleep(t *testing.T) {
	f := newProcessExecFixture(t)
	f.start("sleep 5")

	f.waitForStatusAndNoError(Running)

	time.Sleep(5 * time.Second)

	f.assertCmdSucceeds()
}

func TestPrintsLogs(t *testing.T) {
	f := newProcessExecFixture(t)
	f.start("echo testing123456")
	f.assertCmdSucceeds()
	f.assertLogContains("testing123456")
}

func TestHandlesFailures(t *testing.T) {
	f := newProcessExecFixture(t)
	f.start("false")

	f.waitForError()
}

type processExecFixture struct {
	t          *testing.T
	ctx        context.Context
	execer     *processExecer
	testWriter *strings.Builder
	statusCh   chan Status
}

func newProcessExecFixture(t *testing.T) *processExecFixture {
	execer := NewProcessExecer()
	ctx := context.Background()
	testWriter := &strings.Builder{}
	statusCh := make(chan Status)

	return &processExecFixture{
		t:          t,
		ctx:        ctx,
		execer:     execer,
		testWriter: testWriter,
		statusCh:   statusCh,
	}
}

func (f *processExecFixture) start(cmd string) {
	c := model.ToShellCmd(cmd)
	f.execer.Start(f.ctx, c, f.testWriter, f.statusCh)
}

func (f *processExecFixture) assertCmdSucceeds() {
	f.waitForStatusAndNoError(Done)
}

func (f *processExecFixture) waitForStatusAndNoError(expectedStatus Status) {
	for {
		select {
		case status := <-f.statusCh:
			if expectedStatus == status {
				return
			}
			if expectedStatus == Error {
				f.t.Error("Unexpected Error status")
				return
			}
		case <-time.After(10 * time.Second):
			f.t.Fatal("Timed out waiting for cmd status")
		}
	}
}

func (f *processExecFixture) assertLogContains(s string) {
	assert.Contains(f.t, f.testWriter.String(), s)
}

func (f *processExecFixture) waitForError() {
	for {
		select {
		case status := <-f.statusCh:
			if status == Error {
				return
			}
		case <-time.After(10 * time.Second):
			f.t.Fatal("Timed out waiting for error")
		}
	}
}
