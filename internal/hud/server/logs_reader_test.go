package server

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/hud"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

var alphabet = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot",
	"golf", "hotel", "igloo", "juliette", "kilo", "lima", "mike", "november",
	"oscar", "papa", "quebec", "romeo", "sierra", "tango", "uniform", "victor",
	"whiskey", "xavier", "yankee", "zulu"}

type expectedLine struct {
	prefix  string // ~manifestName (leave blank for "global" / unprefixed)
	message string
}

func TestLogStreamerPrintsLogs(t *testing.T) {
	f := newLogStreamerFixture(t)

	view := f.newViewWithLogsForManifest(alphabet[:4], "foo", 0)
	f.handle(view)

	expected := f.expectedLinesWithPrefix(alphabet[:4], "foo")

	f.assertExpectedLogLines(expected)
}

func TestHandleEmptyView(t *testing.T) {
	f := newLogStreamerFixture(t)
	f.handle(&proto_webview.View{})
	f.assertExpectedLogLines([]expectedLine{expectedLine{}}) // Always end in a newline
}

func TestLogStreamerPrefixing(t *testing.T) {
	f := newLogStreamerFixture(t)
	manifestNames := []string{"foo", "", "foo", "bar"}

	view := f.newViewWithLogsForManifests(alphabet[:4], manifestNames, 0)
	f.handle(view)

	expected := f.expectedLinesWithPrefixes(alphabet[:4], manifestNames)

	f.assertExpectedLogLines(expected)
}

func TestLogStreamerDoesNotPrintResentLogs(t *testing.T) {
	f := newLogStreamerFixture(t)

	view := f.newViewWithLogsForManifest(alphabet[:4], "foo", 0)
	f.handle(view)

	view = f.newViewWithLogsForManifest(alphabet[:5], "foo", 0)
	f.handle(view)

	expected := f.expectedLinesWithPrefix(alphabet[:5], "foo")

	f.assertExpectedLogLines(expected)
}

func TestLogStreamerCheckpointHandling(t *testing.T) {
	// the client might attach after the server has truncated, so the first "FromCheckpoint" it sees
	// won't always be 0
	for _, initialOffset := range []int32{0, 1000} {
		t.Run(fmt.Sprintf("Offset-%d", initialOffset), func(t *testing.T) {
			f := newLogStreamerFixture(t)
			resourceNames := []string{
				"foo", "", "foo", "bar",
				"bar", "bar", "", "bar",
				"", "foo", "bar", "baz"}
			view := f.newViewWithLogsForManifests(alphabet[:4], resourceNames[:4], initialOffset)
			f.handle(view)

			view = f.newViewWithLogsForManifests(alphabet[4:8], resourceNames[4:8], view.LogList.ToCheckpoint)
			f.handle(view)

			view = f.newViewWithLogsForManifests(alphabet[8:12], resourceNames[8:12], view.LogList.ToCheckpoint)
			f.handle(view)

			expected := f.expectedLinesWithPrefixes(alphabet[:12], resourceNames[:12])
			f.assertExpectedLogLines(expected)
		})
	}
}

func TestLogStreamerFiltersOnResourceNamesSingle(t *testing.T) {
	f := newLogStreamerFixture(t).withResourceNames("foo")
	manifestNames := []string{"foo", "", "foo", "bar"}
	view := f.newViewWithLogsForManifests(alphabet[:4], manifestNames, 0)
	f.handle(view)

	// Expect no prefix b/c we're filtering for a single resource, so prefixing is redundant
	expected := f.expectedLinesWithPrefix([]string{"alpha", "charlie"}, "")
	f.assertExpectedLogLines(expected)
}

func TestLogStreamerFiltersOnResourceNamesMultiple(t *testing.T) {
	f := newLogStreamerFixture(t).withResourceNames("foo", "baz")
	manifestNames := []string{"foo", "", "foo", "bar", "baz", "bar", "baz", "foo"}
	view := f.newViewWithLogsForManifests(alphabet[:8], manifestNames, 0)
	f.handle(view)

	expected := f.expectedLinesWithPrefixes(
		[]string{"alpha", "charlie", "echo", "golf", "hotel"}, []string{"foo", "foo", "baz", "baz", "foo"})
	f.assertExpectedLogLines(expected)
}

func TestLogStreamerCheckpointHandlingWithFiltering(t *testing.T) {
	f := newLogStreamerFixture(t).withResourceNames("foo", "baz")
	view := f.newViewWithLogsForManifests(alphabet[:4], []string{"foo", "", "foo", "bar"}, 0)
	f.handle(view)

	view = f.newViewWithLogsForManifests(alphabet[4:8], []string{"bar", "bar", "", "bar"}, view.LogList.ToCheckpoint)
	f.handle(view)

	view = f.newViewWithLogsForManifests(alphabet[8:12], []string{"", "foo", "bar", "baz"}, view.LogList.ToCheckpoint)
	f.handle(view)

	expected := f.expectedLinesWithPrefixes(
		[]string{"alpha", "charlie", "juliette", "lima"}, []string{"foo", "foo", "foo", "baz"})
	f.assertExpectedLogLines(expected)
}

type logStreamerFixture struct {
	t          *testing.T
	fakeStdout *bytes.Buffer
	printer    *hud.IncrementalPrinter
	ls         *LogStreamer
}

func newLogStreamerFixture(t *testing.T) *logStreamerFixture {
	fakeStdout := &bytes.Buffer{}
	printer := hud.NewIncrementalPrinter(hud.Stdout(fakeStdout))
	filter := hud.NewLogFilter(
		hud.FilterSourceAll,
		hud.FilterResources{},
		hud.FilterLevel(logger.InfoLvl))
	return &logStreamerFixture{
		t:          t,
		fakeStdout: fakeStdout,
		printer:    printer,
		ls:         NewLogStreamer(filter, printer),
	}
}

func (f *logStreamerFixture) withResourceNames(resourceNames ...string) *logStreamerFixture {
	resources := []model.ManifestName{}
	for _, rn := range resourceNames {
		resources = append(resources, model.ManifestName(rn))
	}
	f.ls.filter = hud.NewLogFilter(
		hud.FilterSourceAll,
		hud.FilterResources(resources),
		hud.FilterLevel(logger.InfoLvl))
	return f
}

func (f *logStreamerFixture) handle(view *proto_webview.View) {
	err := f.ls.Handle(view)
	require.NoError(f.t, err)
}

func (f *logStreamerFixture) newViewWithLogsForManifest(messages []string, manifestName string, fromChkpt int32) *proto_webview.View {
	dummyManifestNames := make([]string, len(messages))
	for i := 0; i < len(messages); i++ {
		dummyManifestNames[i] = manifestName
	}
	return f.newViewWithLogsForManifests(messages, dummyManifestNames, fromChkpt)
}

func (f *logStreamerFixture) newViewWithLogsForManifests(messages []string, manifestNames []string, fromChkpt int32) *proto_webview.View {
	segs := f.segments(messages, manifestNames)
	spans := f.spans(manifestNames, nil)

	return &proto_webview.View{
		LogList: &proto_webview.LogList{
			Spans:          spans,
			Segments:       segs,
			FromCheckpoint: fromChkpt,
			ToCheckpoint:   fromChkpt + int32(len(segs)),
		},
	}
}

func (f *logStreamerFixture) segments(messages []string, manifestNames []string) []*proto_webview.LogSegment {
	if len(messages) != len(manifestNames) {
		f.t.Fatalf("Need same number of messages and manifestNames (got %d and %d)",
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

	return segs
}

func (f *logStreamerFixture) spans(manifestNames []string,
	existingSpans map[string]*proto_webview.LogSpan) map[string]*proto_webview.LogSpan {

	if existingSpans == nil {
		existingSpans = make(map[string]*proto_webview.LogSpan)
	}

	for _, mn := range manifestNames {
		existingSpans[spanID(mn)] = &proto_webview.LogSpan{ManifestName: mn}
	}

	return existingSpans
}

func (f *logStreamerFixture) assertExpectedLogLines(expectedLines []expectedLine) {
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

func (f *logStreamerFixture) expectedLinesWithPrefix(messages []string, prefix string) []expectedLine {
	expected := make([]expectedLine, len(messages))
	for i, msg := range messages {
		expected[i] = expectedLine{prefix, msg}
	}
	return expected
}

func (f *logStreamerFixture) expectedLinesWithPrefixes(messages []string, prefixes []string) []expectedLine {
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
