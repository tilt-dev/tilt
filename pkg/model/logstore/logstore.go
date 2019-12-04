package logstore

import (
	"fmt"
	"strings"
	"time"

	"github.com/windmilleng/tilt/pkg/model"
)

// At this limit, with one resource having a 120k log, render time was ~20ms and CPU usage was ~70% on an MBP.
// 70% still isn't great when tilt doesn't really have any necessary work to do, but at least it's usable.
// A render time of ~40ms was about when the interface started being noticeably laggy to me.
const maxLogLengthInBytes = 120

// After a log hits its limit, we need to truncate it to keep it small
// we do this by cutting a big chunk at a time, so that we have rarer, larger changes, instead of
// a small change every time new data is written to the log
// https://github.com/windmilleng/tilt/issues/1935#issuecomment-531390353
const logTruncationTarget = maxLogLengthInBytes / 2

const newlineByte = byte('\n')

type Span struct {
	ManifestName     model.ManifestName
	LastSegmentIndex int
}

func (s *Span) Clone() *Span {
	clone := *s
	return &clone
}

type SpanID string

type LogSegment struct {
	SpanID SpanID
	Time   time.Time
	Text   []byte

	// Continues a line from a previous segment.
	ContinuesLine bool
}

func (l LogSegment) StartsLine() bool {
	return !l.ContinuesLine
}

func (l LogSegment) IsComplete() bool {
	segmentLen := len(l.Text)
	return segmentLen > 0 && l.Text[segmentLen-1] == newlineByte
}

func (l LogSegment) Len() int {
	return len(l.Text)
}

func (l LogSegment) String() string {
	return string(l.Text)
}

func segmentsFromBytes(spanID SpanID, time time.Time, bs []byte) []LogSegment {
	segments := []LogSegment{}
	lastBreak := 0
	for i, b := range bs {
		if b == newlineByte {
			segments = append(segments, LogSegment{
				SpanID: spanID,
				Time:   time,
				Text:   bs[lastBreak : i+1],
			})
			lastBreak = i + 1
		}
	}
	if lastBreak < len(bs) {
		segments = append(segments, LogSegment{
			SpanID: spanID,
			Time:   time,
			Text:   bs[lastBreak:],
		})
	}
	return segments
}

type LogEvent interface {
	Message() []byte
	Time() time.Time

	// Ideally, all logs should be associated with a source.
	//
	// In practice, not all logs have an obvious source identifier,
	// so this might be empty.
	//
	// Right now, that source is a ManifestName. But in the future,
	// this might make more sense as another kind of identifier (like SpanID).
	//
	// (As of this writing, we have TargetID as an abstract build-time
	// source identifier, but no generic run-time source identifier)
	Source() model.ManifestName
}

type LogStore struct {
	// A Span is a grouping of logs by their source.
	// The term "Span" is taken from opentracing, and has similar associations.
	spans map[SpanID]*Span

	// We store logs as an append-only sequence of segments.
	// Once a segment has been added, it should not be modified.
	segments []LogSegment

	// The number of bytes stored in this logstore. This is redundant bookkeeping so
	// that we don't need to recompute it each time.
	len int
}

func NewLogStore() *LogStore {
	return &LogStore{
		spans:    make(map[SpanID]*Span),
		segments: []LogSegment{},
		len:      0,
	}
}

func (s *LogStore) Append(le LogEvent, secrets model.SecretSet) {
	// TODO(nick): This is a white lie until we have the code instrumented
	// to create real span ids.
	spanID := SpanID(le.Source())
	span, ok := s.spans[spanID]
	if !ok {
		span = &Span{ManifestName: le.Source(), LastSegmentIndex: -1}
		s.spans[spanID] = span
	}

	msg := secrets.Scrub(le.Message())

	isStartingNewLine := false
	if span.LastSegmentIndex == -1 {
		isStartingNewLine = true
	} else if s.segments[span.LastSegmentIndex].IsComplete() {
		isStartingNewLine = true
	}

	added := segmentsFromBytes(spanID, le.Time(), msg)
	if len(added) == 0 {
		return
	}

	added[0].ContinuesLine = !isStartingNewLine

	s.segments = append(s.segments, added...)
	span.LastSegmentIndex = len(s.segments) - 1

	s.len += len(msg)
	s.ensureMaxLength()
}

// Get at most N lines from the tail of the log.
func (s *LogStore) Tail(n int) *LogStore {
	if n <= 0 {
		return NewLogStore()
	}

	// Traverse backwards until we have n lines.
	remaining := n
	start := len(s.segments) - 1
	for ; start >= 0; start-- {
		if s.segments[start].StartsLine() {
			remaining--
			if remaining <= 0 {
				break
			}
		}
	}

	if remaining > 0 {
		// If there aren't enough lines, just return the whole store.
		return s
	}

	startedSpans := make(map[SpanID]bool)
	newSegments := []LogSegment{}
	for i := start; i < len(s.segments); i++ {
		segment := s.segments[i]
		spanID := segment.SpanID

		if !segment.StartsLine() && !startedSpans[spanID] {
			// Skip any segments that start on lines from before the Tail started.
			continue
		}
		newSegments = append(newSegments, segment)
		startedSpans[spanID] = true
	}

	newSpans := make(map[SpanID]*Span)
	for _, segment := range newSegments {
		newSpans[segment.SpanID] = s.spans[segment.SpanID].Clone()
	}
	newStore := &LogStore{spans: newSpans, segments: newSegments}
	newStore.recomputeDerivedValues()
	return newStore
}

func (s *LogStore) recomputeDerivedValues() {
	s.len = s.computeLen()

	// Reset the last segment index so we can rebuild them from scratch.
	for _, span := range s.spans {
		span.LastSegmentIndex = -1
	}

	// Rebuild information about line continuations.
	for i, segment := range s.segments {
		spanID := segment.SpanID
		span := s.spans[spanID]

		isStartingNewLine := false
		if span.LastSegmentIndex == -1 {
			isStartingNewLine = true
		} else if s.segments[span.LastSegmentIndex].IsComplete() {
			isStartingNewLine = true
		}

		s.segments[i].ContinuesLine = !isStartingNewLine
		span.LastSegmentIndex = i
	}
}

func (s *LogStore) String() string {
	sb := strings.Builder{}
	lastLineCompleted := false

	// We want to print the log line-by-line, but we don't actually store the logs
	// line-by-line. We store them as segments.
	//
	// This means we need to:
	// 1) At segment x,
	// 2) If x starts a new line, print it, then run ahead to print the rest of the line
	//    until the entire line is consumed.
	// 3) If x does not start a new line, skip it, because we assume it was handled
	//    in a previous line.
	//
	// This can have some O(n^2) perf characteristics in the worst case, but
	// for normal inputs should be fine.
	for i, segment := range s.segments {
		if !segment.StartsLine() {
			continue
		}

		// If the last segment never completed, print a newline now, so that the
		// logs from different sources don't blend together.
		if i > 0 && !lastLineCompleted {
			sb.WriteString("\n")
		}

		spanID := segment.SpanID
		span := s.spans[spanID]
		if span.ManifestName != "" {
			sb.WriteString(fmt.Sprintf("%s | ", span.ManifestName))
		}
		sb.WriteString(string(segment.Text))

		// If this segment is not complete, run ahead and try to complete it.
		if segment.IsComplete() {
			lastLineCompleted = true
			continue
		}

		lastLineCompleted = false
		for currentIndex := i + 1; currentIndex <= span.LastSegmentIndex; currentIndex++ {
			currentSeg := s.segments[currentIndex]
			if currentSeg.SpanID != spanID {
				continue
			}

			sb.WriteString(string(currentSeg.Text))
			if currentSeg.IsComplete() {
				lastLineCompleted = true
				break
			}
		}
	}
	return sb.String()
}

func (s *LogStore) computeLen() int {
	result := 0
	for _, segment := range s.segments {
		result += segment.Len()
	}
	return result
}

func (s *LogStore) ensureMaxLength() {
	if s.len <= maxLogLengthInBytes {
		return
	}

	// Figure out where we have to truncate.
	bytesSpent := 0
	truncationIndex := -1
	for i := len(s.segments) - 1; i >= 0; i-- {
		segment := s.segments[i]
		bytesSpent += segment.Len()
		if truncationIndex == -1 && bytesSpent > logTruncationTarget {
			truncationIndex = i + 1
		}
		if bytesSpent > maxLogLengthInBytes {
			s.segments = s.segments[truncationIndex:]
			s.recomputeDerivedValues()
			return
		}
	}
}
