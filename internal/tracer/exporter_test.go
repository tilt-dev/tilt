package tracer

import (
	"testing"
)

func TestSimple(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	sd1 := sd(1)
	f.export(sd1)

	f.assertConsumeSpans(sd1)

	sd2 := sd(2)
	f.export(sd2)
	f.assertConsumeSpans(sd2)

	f.assertEmpty()
}

func TestReject(t *testing.T) {
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
	f.assertEmpty()
}

func TestString(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	sd := &exporttrace.SpanData{
		SpanContext: core.SpanContext{
			SpanID: core.SpanIDFromHex("00f067aa0ba902b7"),
		},
		Name: "foo",
	}

	f.export(sd)
	s := f.consumeString()
	expected := ``
	if s != expected {
		t.Fatalf("got %v; expected %v", s, expected)
	}

	f.assertEmpty()
}

type fixture struct {
	t *testing.T
}

func newFixture(t *testing.T) *fixture {
	return &fixture{
		t: t,
	}
}

func (f *fixture) tearDown() {
}

func sd(id int) *exporttrace.SpanData {
	return &exporttrace.SpanData{
		SpanContext: core.SpanContext{
			SpanID: idFromInt(id),
		},
	}
}

func idFromInt(id int) (r core.SpanID) {
	r[7] = r % 256
	r[6] = r / 256
	return r
}
