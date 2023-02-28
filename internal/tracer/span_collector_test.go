package tracer

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestExporterSimple(t *testing.T) {
	f := newFixture(t)

	sd1 := sd(1)
	f.export(sd1)

	f.assertConsumeSpans(sd1)

	sd2 := sd(2)
	f.export(sd2)
	f.assertConsumeSpans(sd2)
}

func TestExporterReject(t *testing.T) {
	f := newFixture(t)

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

	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sd := tracetest.SpanStub{
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			SpanID: spanID,
		}),
		Name: "foo",
	}.Snapshot()

	f.export(sd)
	s, _ := f.getSpanText()
	// N.B. we add a `tilt.usage/` prefix to the span name during export
	expected := `{"SpanContext":{"TraceID":"00000000000000000000000000000000","SpanID":"00f067aa0ba902b7","TraceFlags":0},"ParentSpanID":"0000000000000000","SpanKind":0,"Name":"tilt.dev/usage/foo","StartTime":"0001-01-01T00:00:00Z","EndTime":"0001-01-01T00:00:00Z","Attributes":null,"MessageEvents":null,"Links":null,"Status":0,"HasRemoteParent":false,"DroppedAttributeCount":0,"DroppedMessageEventCount":0,"DroppedLinkCount":0,"ChildSpanCount":0}
`

	require.JSONEq(t, expected, s, "spans did not match")
}

func TestExporterTrims(t *testing.T) {
	f := newFixture(t)

	var sds []sdktrace.ReadOnlySpan
	for i := 0; i < 2048; i++ {
		sdi := sd(i)
		sds = append(sds, sdi)
		f.export(sdi)
	}

	f.assertConsumeSpans(sds[1024:]...)
}

func TestExporterStartsEmpty(t *testing.T) {
	f := newFixture(t)

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
	ret := &fixture{
		t:   t,
		ctx: ctx,
		sc:  sc,
	}

	t.Cleanup(ret.tearDown)
	return ret
}

func (f *fixture) tearDown() {
	f.t.Helper()
	f.assertEmpty()
	require.NoError(f.t, f.sc.Shutdown(f.ctx))
	require.NoError(f.t, f.sc.Close())
}

func (f *fixture) export(sd sdktrace.ReadOnlySpan) {
	f.t.Helper()
	require.NoError(f.t, f.sc.ExportSpans(f.ctx, []sdktrace.ReadOnlySpan{sd}))
}

func (f *fixture) assertConsumeSpans(expected ...sdktrace.ReadOnlySpan) {
	f.t.Helper()
	actual, _ := f.getSpans()

	f.assertSpansEqual(expected, actual)
}

func (f *fixture) assertRejectSpans(expected ...sdktrace.ReadOnlySpan) {
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

func (f *fixture) assertSpansEqual(expected []sdktrace.ReadOnlySpan, actual []sdktrace.ReadOnlySpan) {
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
		assert.JSONEq(f.t, string(exJSON), string(actJSON), "unequal spans")
	}
}

func (f *fixture) getSpanText() (string, func()) {
	f.t.Helper()
	r, rejectFn, err := f.sc.GetOutgoingSpans()
	if err != nil {
		f.t.Fatalf("unexpected error %v", err)
	}

	bs, err := io.ReadAll(r)
	if err != nil {
		f.t.Fatalf("unexpected error %v", err)
	}

	return string(bs), rejectFn
}

func (f *fixture) getSpans() ([]sdktrace.ReadOnlySpan, func()) {
	f.t.Helper()
	s, rejectFn := f.getSpanText()
	r := strings.NewReader(s)
	dec := json.NewDecoder(r)

	var result []sdktrace.ReadOnlySpan

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

func sdFromData(data SpanDataFromJSON) sdktrace.ReadOnlySpan {
	spanID, _ := trace.SpanIDFromHex(data.SpanContext.SpanID)

	return tracetest.SpanStub{
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			SpanID: spanID,
		}),
	}.Snapshot()
}

func sd(id int) sdktrace.ReadOnlySpan {
	return tracetest.SpanStub{
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			SpanID: idFromInt(id),
		}),
	}.Snapshot()
}

func idFromInt(id int) (r trace.SpanID) {
	r[7] = uint8(id % 256)
	r[6] = uint8(id / 256)
	return r
}
