package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/model"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
)

func TestTelNoScriptTimeIsUpNoInvocation(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()

	f.run()

	f.assertNoInvocation()
}

func TestTelNoScriptTimeIsNotUpNoInvocation(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.setLastRun(t1)
	f.run()

	f.assertNoInvoation()
}

func TestTelScriptTimeIsNotUpNoInvocation(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.workCmd()
	f.setLastRun(t1)
	f.run()

	f.assertNoInvocation()
}

func TestTelScriptTimeIsUpNoSpansNoInvocation(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.spans = nil
	f.workCmd()
	f.setLastRun(t1)
	f.run()

	f.assertNoInvocation()
}

func TestTelScriptTimeIsUpShouldDeleteFileAndSetTime(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.workCmd()
	f.run()

	f.assertNoLogs()
	f.assertFileUpdated()
	f.assertTelemetryScriptRanAtIs(t1)
	f.assertNoSpans()
}

func TestTelScriptFailsTimeIsUpShouldDeleteFileAndSetTime(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.failCmd()
	f.run()

	f.assertLog("exit status 1")
	f.assertSpansPresent()
	f.assertTelemetryScriptRanAtIs(t1)
}

type tcFixture struct {
	t         *testing.T
	ctx       context.Context
	temp      *tempdir.TempDirFixture
	clock     fakeClock
	st        *store.TestingStore
	cmd       string
	lastRun   time.Time
	spans []*exporttrace.SpanData
	exporter  *tracer.Exporter
}

func newTCFixture(t *testing.T) *tcFixture {
	temp := tempdir.NewTempDirFixture(t)
	owd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	st := store.NewTestingStore()

	ctx := context.Background()

	// this is a script that writes stdin to a file so we can assert on it
	temp.WriteFile("testscript.sh", fmt.Sprintf(`#!/bin/bash
cat > %s`, temp.JoinPath("scriptstdin")))

	return &tcFixture{
		t:     t,
		ctx:   ctx,
		temp:  temp,
		clock: fakeClock{now: time.Unix(1551202573, 0)},
		st:    st,
		exporter: 	tracer.NewExporter(ctx),
		spans: []*exporttrace.SpanData{&exporttrace.SpanData{}},
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
	for _, sd := range tcf.spans {
		exporter.OnEnd(sd)
	}

	
	tcf.st.SetState(store.EngineState{
		TelemetryStatus: model.TelemetryStatus{
			LastRunAt: tcf.lastRun,
		},
		TelemetryCmd: model.ToShellCmd(tcf.cmd)})

	tc := NewController(tcf.clock, spans)
	tc.OnChange(tcf.ctx, tcf.st)
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
}

func (tcf *tcFixture) getActions() []store.Action {
	return tcf.st.Actions
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }
