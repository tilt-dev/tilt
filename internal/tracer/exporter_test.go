package tracer

import (
	"context"
	"encoding/json"
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
	s, ch := f.getSpanText()
	ch <- true
	expected := `{"SpanContext":{"TraceID":"00000000000000000000000000000000","SpanID":"00f067aa0ba902b7","TraceFlags":0},"ParentSpanID":"0000000000000000","SpanKind":0,"Name":"foo","StartTime":"0001-01-01T00:00:00Z","EndTime":"0001-01-01T00:00:00Z","Attributes":null,"MessageEvents":null,"Links":null,"Status":0,"HasRemoteParent":false,"DroppedAttributeCount":0,"DroppedMessageEventCount":0,"DroppedLinkCount":0,"ChildSpanCount":0}
`
	if *s != expected {
		t.Fatalf("got %v; expected %v", *s, expected)
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
	ex  *Exporter
}

func newFixture(t *testing.T) *fixture {
	ctx := context.Background()
	ex := NewExporter(ctx)
	return &fixture{
		t:   t,
		ctx: ctx,
		ex:  ex,
	}
}

func (f *fixture) tearDown() {
	f.assertEmpty()
}

func (f *fixture) export(sd *exporttrace.SpanData) {
	f.ex.OnStart(sd)
	f.ex.OnEnd(sd)
}

func (f *fixture) assertConsumeSpans(expected ...*exporttrace.SpanData) {
	actual, ch := f.getSpans()
	ch <- true

	f.assertSpansEqual(expected, actual)
}

func (f *fixture) assertRejectSpans(expected ...*exporttrace.SpanData) {
	actual, ch := f.getSpans()
	ch <- false

	f.assertSpansEqual(expected, actual)
}

func (f *fixture) assertEmpty() {
	actual, ch := f.getSpans()
	ch <- true

	f.assertSpansEqual(nil, actual)
}

func (f *fixture) assertSpansEqual(expected []*exporttrace.SpanData, actual []*exporttrace.SpanData) {
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

func (f *fixture) getSpanText() (*string, chan<- bool) {
	r, ch, err := f.ex.GetOutgoingSpans()
	if err != nil {
		f.t.Fatalf("unexpected error %v", err)
	}

	if r == nil {
		return nil, ch
	}

	bs, err := ioutil.ReadAll(r)
	if err != nil {
		f.t.Fatalf("unexpected error %v", err)
	}

	result := string(bs)
	return &result, ch
}

func (f *fixture) getSpans() ([]*exporttrace.SpanData, chan<- bool) {
	s, ch := f.getSpanText()
	if s == nil {
		return nil, ch
	}
	r := strings.NewReader(*s)
	dec := json.NewDecoder(r)

	var result []*exporttrace.SpanData

	for dec.More() {
		var data SpanDataFromJSON
		if err := dec.Decode(&data); err != nil {
			f.t.Fatalf("unexpected error %v %q", err, *s)
		}
		result = append(result, sdFromData(data))
	}

	if len(result) == 0 {
		f.t.Fatalf("Got an empty string from a non-nil Reader")
	}

	return result, ch
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
