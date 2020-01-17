package telemetry

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/tracer"
	"github.com/windmilleng/tilt/pkg/model"
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

	f.assertNoInvocation()
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

func TestTelScriptTimeIsUpShouldClearSpansAndSetTime(t *testing.T) {
	f := newTCFixture(t)
	defer f.teardown()
	t1 := time.Now()
	f.clock.now = t1

	f.workCmd()
	f.run()

	f.assertInvocation()
	f.assertCmdOutput(`{"SpanContext":{"TraceID":"00000000000000000000000000000000","SpanID":"0000000000000000","TraceFlags":0},"ParentSpanID":"0000000000000000","SpanKind":0,"Name":"","StartTime":"0001-01-01T00:00:00Z","EndTime":"0001-01-01T00:00:00Z","Attributes":null,"MessageEvents":null,"Links":null,"Status":0,"HasRemoteParent":false,"DroppedAttributeCount":0,"DroppedMessageEventCount":0,"DroppedLinkCount":0,"ChildSpanCount":0}
`)
	f.assertNoLogs()
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
	t          *testing.T
	ctx        context.Context
	temp       *tempdir.TempDirFixture
	clock      fakeClock
	st         *store.TestingStore
	cmd        string
	lastRun    time.Time
	spans      []*exporttrace.SpanData
	sc         *tracer.SpanCollector
	controller *Controller
}

func newTCFixture(t *testing.T) *tcFixture {
	temp := tempdir.NewTempDirFixture(t)

	st := store.NewTestingStore()

	ctx := context.Background()

	return &tcFixture{
		t:     t,
		ctx:   ctx,
		temp:  temp,
		clock: fakeClock{now: time.Unix(1551202573, 0)},
		st:    st,
		sc:    tracer.NewSpanCollector(ctx),
		spans: []*exporttrace.SpanData{&exporttrace.SpanData{}},
	}
}

func (tcf *tcFixture) workCmd() {
	tcf.cmd = fmt.Sprintf("touch %s; cat > %s", tcf.temp.JoinPath("ran.txt"), tcf.temp.JoinPath("scriptstdout"))
}

func (tcf *tcFixture) failCmd() {
	tcf.cmd = fmt.Sprintf("touch %s; false", tcf.temp.JoinPath("ran.txt"))
}

func (tcf *tcFixture) assertNoInvocation() {
	_, err := os.Stat(tcf.temp.JoinPath("ran.txt"))
	if !os.IsNotExist(err) {
		tcf.t.Fatalf("expected ran.txt to not exist")
	}
}

func (tcf *tcFixture) assertInvocation() {
	_, err := os.Stat(tcf.temp.JoinPath("ran.txt"))
	if err != nil {
		tcf.t.Fatalf("error stat'ing ran.txt: %v", err)
	}
}

func (tcf *tcFixture) setLastRun(t time.Time) {
	tcf.lastRun = t
}

func (tcf *tcFixture) run() {
	for _, sd := range tcf.spans {
		tcf.sc.OnEnd(sd)
	}

	ts := model.TelemetrySettings{
		Cmd:     model.ToShellCmd(tcf.cmd),
		Workdir: tcf.temp.Path(),
	}
	tcf.st.SetState(store.EngineState{
		TelemetrySettings: ts,
	})

	tc := NewController(tcf.clock, tcf.sc)
	tc.lastRunAt = tcf.lastRun
	tcf.controller = tc
	tc.OnChange(tcf.ctx, tcf.st)
}

func (tcf *tcFixture) assertNoLogs() {
	actions := tcf.st.Actions()
	for _, a := range actions {
		if la, ok := a.(store.LogAction); ok {
			tcf.t.Errorf("Expected no LogActions but found: %v", la)
		}
	}
}

func (tcf *tcFixture) assertLog(logMsg string) {
	actions := tcf.st.Actions()
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
	assert.Equal(tcf.t, t, tcf.controller.lastRunAt)
}

func (tcf *tcFixture) assertCmdOutput(expected string) {
	bs, err := ioutil.ReadFile(tcf.temp.JoinPath("scriptstdout"))
	if err != nil {
		tcf.t.Fatal(err)
	}

	assert.Equal(tcf.t, expected, string(bs))
}

func (tcf *tcFixture) assertSpansPresent() {
	_, _, err := tcf.sc.GetOutgoingSpans()
	if err != nil {
		tcf.t.Fatalf("error getting spans: %v", err)
	}
}

func (tcf *tcFixture) assertNoSpans() {
	r, _, err := tcf.sc.GetOutgoingSpans()
	if err != io.EOF {
		tcf.t.Fatalf("Didn't get EOF for spans: %v %v", r, err)
	}
}

func (tcf *tcFixture) teardown() {
	defer tcf.temp.TearDown()
}

func (tcf *tcFixture) getActions() []store.Action {
	return tcf.st.Actions()
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }
