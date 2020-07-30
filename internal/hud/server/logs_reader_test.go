package server

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/hud"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

const fooSpanID = "spanIDFoo"

var messages = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf"}

type expectedLine struct {
	prefix  string // ~manifestName (leave blank for "global" / unprefixed)
	message string
}

func TestLogStreamerPrintsLogs(t *testing.T) {
	f := newFixture(t)
	view := newViewWithLogs(0)
	err := f.ls.Handle(view)
	require.NoError(t, err)

	expected := f.expectedLinesWithPrefix(messages, "foo")

	f.assertExpectedLogLines(expected)
}

func TestLogStreamerPrintsStartingAtPreviousCheckpoint(t *testing.T) {

}

func TestLogStreamerPrintsNothingIfServerSentNoNewLogs(t *testing.T) {

}

func TestLogStreamerDoesNotPrintResentLogs(t *testing.T) {

}

type fixture struct {
	t          *testing.T
	fakeStdout *bytes.Buffer
	printer    *hud.IncrementalPrinter
	ls         *LogStreamer
}

func newFixture(t *testing.T) *fixture {
	fakeStdout := &bytes.Buffer{}
	printer := hud.NewIncrementalPrinter(hud.Stdout(fakeStdout))
	return &fixture{
		t:          t,
		fakeStdout: fakeStdout,
		printer:    printer,
		ls:         NewLogStreamer(printer),
	}
}

func newViewWithLogs(from int32) proto_webview.View {
	segs := make([]*proto_webview.LogSegment, len(messages))
	for i, msg := range messages {
		segs[i] = &proto_webview.LogSegment{
			SpanId: fooSpanID, // TODO
			Text:   msg + "\n",
			Level:  0, // TODO
		}
	}
	fooSpan := proto_webview.LogSpan{ManifestName: "foo"}

	return proto_webview.View{
		LogList: &proto_webview.LogList{ // TODO: be better
			Spans: map[string]*proto_webview.LogSpan{
				fooSpanID: &fooSpan,
			},
			Segments:       segs,
			FromCheckpoint: from,
			ToCheckpoint:   int32(len(segs)),
		},
	}
}

func (f *fixture) assertExpectedLogLines(expectedLines []expectedLine) {
	out := strings.TrimRight(f.fakeStdout.String(), "\n")
	outLines := strings.Split(out, "\n")
	if len(outLines) != len(expectedLines) {
		f.t.Errorf("Expected %d log lines but got %d", len(expectedLines), len(outLines))
		fmt.Printf("=== Test failed with logs ===\n%s\n", out)
		f.t.FailNow()
	}

	for i, ln := range outLines {
		expected := expectedLines[i]
		assert.True(f.t, strings.Contains(ln, expected.message),
			"expect message %q in line: %q", expected.message, ln)
		if expected.prefix != "" {
			lnTrimmed := strings.TrimSpace(ln)
			assert.True(f.t, strings.HasPrefix(lnTrimmed, expected.prefix),
				"expect prefix %q in line: %q", expected.prefix, lnTrimmed)
		}
	}

	if f.t.Failed() {
		fmt.Printf("=== Test failed with logs ===\n%s\n", out)
	}
}

func (f *fixture) expectedLinesWithPrefix(messages []string, prefix string) []expectedLine {
	expected := make([]expectedLine, len(messages))
	for i, msg := range messages {
		expected[i] = expectedLine{prefix, msg}
	}
	return expected
}

func (f *fixture) expectedLinesWithPrefixes(messages []string, prefixes []string) []expectedLine {
	if len(prefixes) != len(messages) {
		f.t.Fatalf("Must pass same number of prefixes and messages (got %d and %d)",
			len(prefixes), len(messages))
	}
	expected := make([]expectedLine, len(messages))
	for i, msg := range messages {
		expected[i] = expectedLine{prefixes[i], msg}
	}
	return expected
}
