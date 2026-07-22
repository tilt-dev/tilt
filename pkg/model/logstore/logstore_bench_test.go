package logstore

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/pkg/model"
)

// Shaped like a busy dev session: many spans (one per pod/build), with new
// logs arriving on only a few of them between subscriber notifications.
const benchSpanCount = 200

func benchSpanName(i int) model.ManifestName {
	return model.ManifestName(fmt.Sprintf("span-%03d", i))
}

// benchStore builds a store with benchSpanCount spans, each holding
// linesPerSpan complete lines. The byte cap is raised so results measure the
// hot path, not truncation policy.
func benchStore(linesPerSpan int) *LogStore {
	s := NewLogStore()
	s.maxLogLengthInBytes = 500 * 1000 * 1000
	ts := time.Unix(1594000000, 0)
	for line := 0; line < linesPerSpan; line++ {
		for span := 0; span < benchSpanCount; span++ {
			name := benchSpanName(span)
			s.Append(newTestLogEvent(name, ts, fmt.Sprintf("line %d for span %s\n", line, name)), nil)
		}
	}
	return s
}

// The common steady-state notification: a handful of new lines on one span
// since the subscriber's last checkpoint.
func BenchmarkContinuingLinesIncremental(b *testing.B) {
	s := benchStore(10)
	checkpoint := s.Checkpoint()
	ts := time.Unix(1594000100, 0)
	for i := 0; i < 5; i++ {
		s.Append(newTestLogEvent(benchSpanName(0), ts, fmt.Sprintf("new line %d\n", i)), nil)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.ContinuingLines(checkpoint)
	}
}

// A notification that added no logs (e.g. a pure state change): the
// subscriber asks for continuing lines and gets nothing.
func BenchmarkContinuingLinesNoNewLogs(b *testing.B) {
	s := benchStore(10)
	checkpoint := s.Checkpoint()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.ContinuingLines(checkpoint)
	}
}

// The websocket incremental update path: a small batch of new segments
// serialized for the browser or a tilt logs client.
func BenchmarkToLogListIncremental(b *testing.B) {
	s := benchStore(10)
	checkpoint := s.Checkpoint()
	ts := time.Unix(1594000100, 0)
	for i := 0; i < 5; i++ {
		s.Append(newTestLogEvent(benchSpanName(0), ts, fmt.Sprintf("new line %d\n", i)), nil)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.ToLogList(checkpoint)
	}
}

// A full render of the whole store, as on initial web page load or a
// `tilt logs` snapshot: every line goes through the line-builder path with
// manifest prefixes.
func BenchmarkStringFullStore(b *testing.B) {
	s := benchStore(10)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.String()
	}
}

// Log ingestion: one complete line per event, with the default byte cap so
// amortized truncation stays part of the measured cost.
func BenchmarkAppendSingleLine(b *testing.B) {
	s := NewLogStore()
	ts := time.Unix(1594000000, 0)
	event := newTestLogEvent("server", ts, "a fairly typical log line with some payload attached\n")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Append(event, nil)
	}
}

// A multi-line write, e.g. a build log chunk arriving in one event.
func BenchmarkAppendMultiLine(b *testing.B) {
	s := NewLogStore()
	ts := time.Unix(1594000000, 0)
	msg := strings.Repeat("a build output line with some detail in it\n", 100)
	event := newTestLogEvent("build", ts, msg)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Append(event, nil)
	}
}
