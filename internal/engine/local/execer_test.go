package local

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestTrue(t *testing.T) {
	f := newProcessExecFixture(t)
	defer f.tearDown()

	f.start("exit 0")

	f.assertCmdSucceeds()
}

func TestSleep(t *testing.T) {
	f := newProcessExecFixture(t)
	defer f.tearDown()

	cmd := "sleep 1"
	if runtime.GOOS == "windows" {
		// believe it or not, this is the idiomatic way to sleep on windows
		// https://www.ibm.com/support/pages/timeout-command-run-batch-job-exits-immediately-and-returns-error-input-redirection-not-supported-exiting-process-immediately
		cmd = "ping -n 1 127.0.0.1"
	}
	f.start(cmd)

	f.waitForStatusAndNoError(Running)

	time.Sleep(time.Second)

	f.assertCmdSucceeds()
}

func TestPrintsLogs(t *testing.T) {
	f := newProcessExecFixture(t)
	defer f.tearDown()

	f.start("echo testing123456")
	f.assertCmdSucceeds()
	f.assertLogContains("testing123456")
}

func TestHandlesExits(t *testing.T) {
	f := newProcessExecFixture(t)
	defer f.tearDown()

	f.start("exit 1")

	f.waitForError()
	f.assertLogContains("exited with exit code 1")
}

func TestStopsGrandchildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no bash on windows")
	}
	f := newProcessExecFixture(t)
	defer f.tearDown()

	f.start("bash -c '(for i in $(seq 1 20); do echo loop$i; sleep 1; done)'")
	f.waitForStatusAndNoError(Running)

	// wait until there's log output
	timeout := time.After(time.Second)
	for {
		if strings.Contains(f.testWriter.String(), "loop1") {
			break
		}
		select {
		case <-timeout:
			t.Fatal("never saw any process output")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// cancel the context
	f.cancel()
	f.waitForStatusAndNoError(Done)
}

func TestHandlesProcessThatFailsToStart(t *testing.T) {
	f := newProcessExecFixture(t)
	defer f.tearDown()

	f.startMalformedCommand()
	f.waitForError()
	f.assertLogContains("failed to start: ")
}

type processExecFixture struct {
	t          *testing.T
	ctx        context.Context
	cancel     context.CancelFunc
	execer     *processExecer
	testWriter *bufsync.ThreadSafeBuffer
	statusCh   chan statusAndMetadata
}

func newProcessExecFixture(t *testing.T) *processExecFixture {
	execer := NewProcessExecer()
	testWriter := bufsync.NewThreadSafeBuffer()
	ctx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(testWriter)
	ctx, cancel := context.WithCancel(ctx)
	statusCh := make(chan statusAndMetadata)

	return &processExecFixture{
		t:          t,
		ctx:        ctx,
		cancel:     cancel,
		execer:     execer,
		testWriter: testWriter,
		statusCh:   statusCh,
	}
}

func (f *processExecFixture) tearDown() {
	f.cancel()
}

func (f *processExecFixture) startMalformedCommand() {
	c := model.Cmd{Argv: []string{"\""}}
	f.execer.Start(f.ctx, c, f.testWriter, f.statusCh, model.LogSpanID("rt1"))
}

func (f *processExecFixture) start(cmd string) {
	c := model.ToHostCmd(cmd)
	f.execer.Start(f.ctx, c, f.testWriter, f.statusCh, model.LogSpanID("rt1"))
}

func (f *processExecFixture) assertCmdSucceeds() {
	f.waitForError()
	f.assertLogContains("exited with exit code 0")
}

func (f *processExecFixture) waitForStatusAndNoError(expectedStatus status) {
	for {
		select {
		case sm, ok := <-f.statusCh:
			if !ok {
				f.t.Fatal("statusCh closed")
			}
			if expectedStatus == sm.status {
				return
			}
			if expectedStatus == Error {
				f.t.Error("Unexpected Error sm")
				return
			}
		case <-time.After(10 * time.Second):
			f.t.Fatal("Timed out waiting for cmd sm")
		}
	}
}

func (f *processExecFixture) assertLogContains(s string) {
	require.Contains(f.t, f.testWriter.String(), s)
}

func (f *processExecFixture) waitForError() {
	for {
		select {
		case sm := <-f.statusCh:
			if sm.status == Error {
				return
			}
		case <-time.After(10 * time.Second):
			f.t.Fatal("Timed out waiting for error")
		}
	}
}
