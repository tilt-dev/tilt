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

var defaultMessages = []string{"alpha", "bravo", "charlie", "delta"}

type expectedLine struct {
	prefix  string // ~manifestName (leave blank for "global" / unprefixed)
	message string
}

func TestLogStreamerPrintsLogs(t *testing.T) {
	f := newFixture(t)
	view := f.newViewWithLogsForManifest(defaultMessages, "foo")
	f.handle(view)

	expected := f.expectedLinesWithPrefix(defaultMessages, "foo")

	f.assertExpectedLogLines(expected)
}

func TestLogStreamerPrefixing(t *testing.T) {
	f := newFixture(t)
	manifestNames := []string{"foo", "", "foo", "bar"}
	view := f.newViewWithLogsForManifests(defaultMessages, manifestNames)
	f.handle(view)

	expected := f.expectedLinesWithPrefixes(defaultMessages, manifestNames)

	f.assertExpectedLogLines(expected)
}

func TestLogStreamerDoesNotPrintResentLogs(t *testing.T) {
	f := newFixture(t)
	view := f.newViewWithLogsForManifest(defaultMessages, "foo")
	f.handle(view)

	view.LogList.Segments = append(view.LogList.Segments, &proto_webview.LogSegment{
		SpanId: spanID("foo"),
		Text:   "echo",
	})
	view.LogList.ToCheckpoint = int32(len(view.LogList.Segments))
	f.handle(view)

	expected := f.expectedLinesWithPrefix(append(defaultMessages, "echo"), "foo")

	f.assertExpectedLogLines(expected)
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
func (f *fixture) handle(view proto_webview.View) {
	err := f.ls.Handle(view)
	require.NoError(f.t, err)
}

func (f *fixture) newViewWithLogsForManifest(messages []string, manifestName string) proto_webview.View {
	dummyManifestNames := make([]string, len(messages))
	for i := 0; i < len(messages); i++ {
		dummyManifestNames[i] = manifestName
	}
	return f.newViewWithLogsForManifests(messages, dummyManifestNames)
}

func (f *fixture) newViewWithLogsForManifests(messages []string, manifestNames []string) proto_webview.View {
	if len(messages) != len(manifestNames) {
		f.t.Fatalf("Need same number of prefixes and manifestNames (got %d and %d)",
			len(messages), len(manifestNames))
	}

	segs := make([]*proto_webview.LogSegment, len(messages))
	for i, msg := range messages {
		segs[i] = &proto_webview.LogSegment{
			SpanId: spanID(manifestNames[i]),
			Text:   msg + "\n",
			Level:  0, // TODO
		}
	}
	spans := make(map[string]*proto_webview.LogSpan)
	for _, mn := range manifestNames {
		spans[spanID(mn)] = &proto_webview.LogSpan{ManifestName: mn}
	}

	return proto_webview.View{
		LogList: &proto_webview.LogList{ // TODO: be better
			Spans:          spans,
			Segments:       segs,
			FromCheckpoint: 0,
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
		lnTrimmed := strings.TrimSpace(ln)
		expected := expectedLines[i]
		assert.True(f.t, strings.Contains(lnTrimmed, expected.message),
			"expect message %q in line: %q", expected.message, ln)
		if expected.prefix != "" {
			assert.True(f.t, strings.HasPrefix(lnTrimmed, expected.prefix),
				"expect prefix %q in line: %q", expected.prefix, lnTrimmed)
		} else {
			// Expect no prefix
			assert.False(f.t, strings.Contains(lnTrimmed, "|"),
				"expect no prefix but found \"|\" in line: %q", lnTrimmed)
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
		f.t.Fatalf("Need same number of prefixes and messages (got %d and %d)",
			len(prefixes), len(messages))
	}
	expected := make([]expectedLine, len(messages))
	for i, msg := range messages {
		expected[i] = expectedLine{prefixes[i], msg}
	}
	return expected
}

func spanID(mn string) string {
	return fmt.Sprintf("spanID-%s", mn)
}
