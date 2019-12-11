package engine

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestTelNoScriptTimeIsUpShouldNotDeleteFile(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()

	f.run()

	f.assertNoLogs()
	f.assertTelemetryFileEquals("hello world")
}

func TestTelNoScriptTimeIsNotUpShouldNotDeleteFile(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.setLastRun(t1)
	f.run()

	f.assertNoLogs()
	f.assertTelemetryFileEquals("hello world")
}

func TestTelScriptTimeIsNotUpShouldNotDeleteFile(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.workCmd()
	f.setLastRun(t1)
	f.run()

	f.assertNoLogs()
	f.assertTelemetryFileEquals("hello world")
}

func TestTelScriptTimeIsUpShouldDeleteFileAndSetTime(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.workCmd()
	f.run()

	f.assertNoLogs()
	f.assertTelemetryScriptRanAtIs(t1)
	f.assertTelemetryFileIsEmpty()
	f.assertScriptCalledWith("hello world")
}

func TestTelScriptFailsTimeIsUpShouldDeleteFileAndSetTime(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.failCmd()
	f.run()

	f.assertLog("exit status 1")
	f.assertTelemetryFileEquals("hello world")
	f.assertTelemetryScriptRanAtIs(t1)
}

type tcFixture struct {
	t                        *testing.T
	ctx                      context.Context
	temp                     *tempdir.TempDirFixture
	dir                      *dirs.WindmillDir
	lock                     *sync.Mutex
	clock                    fakeClock
	previousWorkingDirectory string
	st                       *store.TestingStore
	cmd                      string
	lastRun                  time.Time
}

func newTCFixture(t *testing.T) *tcFixture {
	temp := tempdir.NewTempDirFixture(t)
	owd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chdir(temp.Path())
	if err != nil {
		t.Fatal(err)
	}
	dir := dirs.NewWindmillDirAt(temp.Path())
	lock := &sync.Mutex{}

	st := store.NewTestingStore()

	ctx := context.Background()

	// this is a script that writes stdin to a file so we can assert on it
	temp.WriteFile("testscript.sh", fmt.Sprintf(`#!/bin/bash
cat > %s`, temp.JoinPath("scriptstdin")))

	return &tcFixture{
		t:                        t,
		ctx:                      ctx,
		temp:                     temp,
		dir:                      dir,
		lock:                     lock,
		clock:                    fakeClock{now: time.Unix(1551202573, 0)},
		previousWorkingDirectory: owd,
		st:                       st,
	}
}

func (tcf *tcFixture) writeToAnalyticsFile(contents string) {
	err := tcf.dir.WriteFile(tracer.OutgoingFilename, contents)
	if err != nil {
		tcf.t.Fatal(err)
	}
}

func (tcf *tcFixture) workCmd() {
	tcf.cmd = fmt.Sprintf("cat > %s", tcf.temp.JoinPath("scriptstdin"))
}

func (tcf *tcFixture) failCmd() {
	tcf.cmd = "false"
}

func (tcf *tcFixture) setLastRun(t time.Time) {
	tcf.lastRun = t
}

func (tcf *tcFixture) run() {
	tcf.writeToAnalyticsFile("hello world")
	tcf.st.SetState(store.EngineState{LastTelemetryScriptRun: tcf.lastRun, TelemetryCmd: model.ToShellCmd(tcf.cmd)})

	tc := NewTelemetryController(tcf.lock, tcf.clock, tcf.dir)
	tc.OnChange(tcf.ctx, tcf.st)
}

func (tcf *tcFixture) assertTelemetryFileIsEmpty() {
	fileContents, err := tcf.dir.ReadFile(tracer.OutgoingFilename)
	if err != nil {
		tcf.t.Fatal(err)
	}

	assert.Empty(tcf.t, fileContents)
}

func (tcf *tcFixture) assertTelemetryFileEquals(contents string) {
	fileContents, err := tcf.dir.ReadFile(tracer.OutgoingFilename)
	if err != nil {
		tcf.t.Fatal(err)
	}

	assert.Equal(tcf.t, contents, fileContents)
}

func (tcf *tcFixture) assertNoLogs() {
	actions := tcf.st.Actions
	for _, a := range actions {
		if la, ok := a.(store.LogAction); ok {
			tcf.t.Errorf("Expected no LogActions but found: %v", la)
		}
	}
}

func (tcf *tcFixture) assertLog(logMsg string) {
	actions := tcf.st.Actions
	for _, a := range actions {
		if la, ok := a.(store.LogAction); ok {
			containsExpected := strings.Contains(string(la.Message()), logMsg)
			if containsExpected {
				return
			}
		}
	}

	tcf.t.Errorf("Couldn't find expected log message %s in %v", logMsg, actions)
}

func (tcf *tcFixture) assertTelemetryScriptRanAtIs(t time.Time) {
	for _, a := range tcf.st.Actions {
		if tsra, ok := a.(TelemetryScriptRanAction); ok {
			assert.True(tcf.t, tsra.At.Equal(t))
			return
		}
	}

	tcf.t.Errorf("Unable to find TelemetryScriptRanAction in %v", tcf.st.Actions)
}

func (tcf *tcFixture) assertScriptCalledWith(expected string) {
	s, err := tcf.dir.ReadFile("scriptstdin")
	if err != nil {
		tcf.t.Fatal(err)
	}
	assert.Equal(tcf.t, expected, s)
}

func (tcf *tcFixture) teardown() {
	defer tcf.temp.TearDown()
	err := os.Chdir(tcf.previousWorkingDirectory)
	if err != nil {
		tcf.t.Fatal(err)
	}
}

func (tcf *tcFixture) getActions() []store.Action {
	return tcf.st.Actions
}
