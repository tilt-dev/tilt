package tracer

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/api/core"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
)

func TestExporterSimple(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	sd1 := sd(1)
	f.export(sd1)

	f.assertConsumeSpans(sd1)

	sd2 := sd(2)
	f.export(sd2)
	f.assertConsumeSpans(sd2)
}

func TestExporterReject(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	sd1 := sd(1)
	f.export(sd1)

	f.assertRejectSpans(sd1)
	f.assertRejectSpans(sd1)
	f.assertRejectSpans(sd1)
	sd2 := sd(2)
	f.export(sd2)
	f.assertRejectSpans(sd1, sd2)
	f.assertConsumeSpans(sd1, sd2)
}

// one test that makes sure the final string we're seeing is reasonable
func TestExporterString(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	spanID, _ := core.SpanIDFromHex("00f067aa0ba902b7")
	sd := &exporttrace.SpanData{
		SpanContext: core.SpanContext{
			SpanID: spanID,
		},
		Name: "foo",
	}

	f.export(sd)
	s, _ := f.getSpanText()
	expected := `{"SpanContext":{"TraceID":"00000000000000000000000000000000","SpanID":"00f067aa0ba902b7","TraceFlags":0},"ParentSpanID":"0000000000000000","SpanKind":0,"Name":"foo","StartTime":"0001-01-01T00:00:00Z","EndTime":"0001-01-01T00:00:00Z","Attributes":null,"MessageEvents":null,"Links":null,"Status":0,"HasRemoteParent":false,"DroppedAttributeCount":0,"DroppedMessageEventCount":0,"DroppedLinkCount":0,"ChildSpanCount":0}
`
	if s != expected {
		t.Fatalf("got %v; expected %v", s, expected)
	}
}

func TestExporterTrims(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	var sds []*exporttrace.SpanData
	for i := 0; i < 2048; i++ {
		sdi := sd(i)
		sds = append(sds, sdi)
		f.export(sdi)
	}

	f.assertConsumeSpans(sds[1024:]...)
}

func TestExporterStartsEmpty(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	f.assertEmpty()
	f.assertEmpty()
	sd1 := sd(1)
	f.export(sd1)
	sd2 := sd(2)
	f.export(sd2)

	f.assertConsumeSpans(sd1, sd2)
	f.assertEmpty()
	f.assertEmpty()
}

type fixture struct {
	t   *testing.T
	ctx context.Context
	sc  *SpanCollector
}

func newFixture(t *testing.T) *fixture {
	ctx := context.Background()
	sc := NewSpanCollector(ctx)
	return &fixture{
		t:   t,
		ctx: ctx,
		sc:  sc,
	}
}

func (f *fixture) tearDown() {
	f.assertEmpty()
	f.sc.Shutdown()
	f.sc.Close()
}

func (f *fixture) export(sd *exporttrace.SpanData) {
	f.sc.OnStart(sd)
	f.sc.OnEnd(sd)
}

func (f *fixture) assertConsumeSpans(expected ...*exporttrace.SpanData) {
	f.t.Helper()
	actual, _ := f.getSpans()

	f.assertSpansEqual(expected, actual)
}

func (f *fixture) assertRejectSpans(expected ...*exporttrace.SpanData) {
	f.t.Helper()
	actual, rejectFn := f.getSpans()
	rejectFn()

	f.assertSpansEqual(expected, actual)
}

func (f *fixture) assertEmpty() {
	f.t.Helper()
	r, _, err := f.sc.GetOutgoingSpans()
	if err != io.EOF {
		f.t.Fatalf("spans not empty: %v %v", r, err)
	}
}

func (f *fixture) assertSpansEqual(expected []*exporttrace.SpanData, actual []*exporttrace.SpanData) {
	f.t.Helper()
	if len(expected) != len(actual) {
		f.t.Fatalf("got %v (len %v); expected %v (len %v)", actual, len(actual), expected, len(expected))
	}

	for i, ex := range expected {
		act := actual[i]

		exJSON, exErr := json.MarshalIndent(ex, "", "  ")
		actJSON, actErr := json.MarshalIndent(act, "", "  ")
		if exErr != nil || actErr != nil {
			f.t.Fatalf("unexpected error %v %v", exErr, actErr)
		}
		if string(exJSON) != string(actJSON) {
			f.t.Fatalf("unequal spans; got:\n%q; expected:\n%q", string(exJSON), string(actJSON))
		}
	}
}

func (f *fixture) getSpanText() (string, func()) {
	f.t.Helper()
	r, rejectFn, err := f.sc.GetOutgoingSpans()
	if err != nil {
		f.t.Fatalf("unexpected error %v", err)
	}

	bs, err := ioutil.ReadAll(r)
	if err != nil {
		f.t.Fatalf("unexpected error %v", err)
	}

	return string(bs), rejectFn
}

func (f *fixture) getSpans() ([]*exporttrace.SpanData, func()) {
	f.t.Helper()
	s, rejectFn := f.getSpanText()
	r := strings.NewReader(s)
	dec := json.NewDecoder(r)

	var result []*exporttrace.SpanData

	for dec.More() {
		var data SpanDataFromJSON
		if err := dec.Decode(&data); err != nil {
			f.t.Fatalf("unexpected error %v %q", err, s)
		}
		result = append(result, sdFromData(data))
	}

	if len(result) == 0 {
		f.t.Fatalf("Got an empty string from a non-nil Reader")
	}

	return result, rejectFn
}

type SpanDataFromJSON struct {
	SpanContext SpanContextFromJSON
}

type SpanContextFromJSON struct {
	SpanID string
}

func sdFromData(data SpanDataFromJSON) *exporttrace.SpanData {
	spanID, _ := core.SpanIDFromHex(data.SpanContext.SpanID)

	return &exporttrace.SpanData{
		SpanContext: core.SpanContext{
			SpanID: spanID,
		},
	}
}

func sd(id int) *exporttrace.SpanData {
	return &exporttrace.SpanData{
		SpanContext: core.SpanContext{
			SpanID: idFromInt(id),
		},
	}
}

func idFromInt(id int) (r core.SpanID) {
	r[7] = uint8(id % 256)
	r[6] = uint8(id / 256)
	return r
}
