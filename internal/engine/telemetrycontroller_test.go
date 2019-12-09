package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
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

var testScript model.Cmd = model.Cmd{
	Argv: []string{"scripts/test.sh"},
}

func TestNoTelScriptTimeIsUpShouldNotDeleteFile(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	tc := f.newTelemetryController()
	ctx := context.Background()

	f.writeToAnalyticsFile("hello world")
	f.st.SetState(store.EngineState{})
	tc.OnChange(ctx, f.st)

	f.assertNoErrors()
	f.assertTelemetryFileEquals("hello world")
}

func TestNoTelScriptTimeIsNotUpShouldNotDeleteFile(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1
	tc := f.newTelemetryController()
	ctx := context.Background()

	f.writeToAnalyticsFile("hello world")
	f.st.SetState(store.EngineState{LastTelemetryScriptRun: t1})
	tc.OnChange(ctx, f.st)

	f.assertNoErrors()
	f.assertTelemetryFileEquals("hello world")
}

func TestTelScriptTimeIsNotUpShouldNotDeleteFile(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1
	tc := f.newTelemetryController()
	ctx := context.Background()

	f.writeToAnalyticsFile("hello world")
	f.st.SetState(store.EngineState{LastTelemetryScriptRun: t1, TelemetryCmd: testScript})
	tc.OnChange(ctx, f.st)

	f.assertNoErrors()
	f.assertTelemetryFileEquals("hello world")
}

func TestTelScriptTimeIsUpShouldDeleteFileAndSetTime(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1
	tc := f.newTelemetryController()
	ctx := context.Background()

	f.writeToAnalyticsFile("hello world")
	f.st.SetState(store.EngineState{TelemetryCmd: testScript})
	tc.OnChange(ctx, f.st)

	f.assertNoErrors()
	f.assertTelemetryScriptRanAtIs(t1)
	f.assertTelemetryFileIsEmpty()
	f.assertScriptCalledWith("hello world")
}

func TestTelScriptFailsTimeIsUpShouldDeleteFileAndSetTime(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1
	tc := f.newTelemetryControllerWithTelemetryScriptThatFails()
	ctx := context.Background()

	f.writeToAnalyticsFile("hello world")
	f.st.SetState(store.EngineState{TelemetryCmd: testScript})
	tc.OnChange(ctx, f.st)

	f.assertError("executable file not found in $PATH")
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
	cmd                      *exec.Cmd
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

func (tcf *tcFixture) newTelemetryController() *TelemetryController {
	return NewTelemetryController(tcf.lock, tcf.clock, tcf.dir, tcf.fakeSuccessExecer)
}

func (tcf *tcFixture) newTelemetryControllerWithTelemetryScriptThatFails() *TelemetryController {
	return NewTelemetryController(tcf.lock, tcf.clock, tcf.dir, tcf.fakeFailExecer)
}

func (tcf *tcFixture) writeToAnalyticsFile(contents string) {
	err := tcf.dir.WriteFile(tracer.OutgoingFilename, contents)
	if err != nil {
		tcf.t.Fatal(err)
	}
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

func (tcf *tcFixture) assertNoErrors() {
	store.AssertNoActionOfType(tcf.t, reflect.TypeOf(store.ErrorAction{}), tcf.getActions)
}

func (tcf *tcFixture) assertError(errMsg string) {
	actions := tcf.st.Actions
	for _, a := range actions {
		if ea, ok := a.(store.ErrorAction); ok {
			containsExpected := strings.Contains(ea.Error.Error(), errMsg)
			if containsExpected {
				return
			}
		}
	}

	tcf.t.Errorf("Couldn't find expected errormsg %s in %v", errMsg, actions)
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

func (tcf *tcFixture) fakeSuccessExecer(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "./testscript.sh")
	tcf.cmd = cmd

	return cmd
}

func (tcf *tcFixture) fakeFailExecer(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "nonsense")
	tcf.cmd = cmd

	return cmd
}

func (tcf *tcFixture) getActions() []store.Action {
	return tcf.st.Actions
}
