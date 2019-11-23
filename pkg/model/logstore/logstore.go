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
	ManifestName  model.ManifestName
	LastLineIndex int
}

func (s *Span) Clone() *Span {
	clone := *s
	return &clone
}

type SpanID string

type Line struct {
	SpanID SpanID
	Time   time.Time
	Text   []byte
}

func (l Line) Append(l2 Line) Line {
	return Line{
		SpanID: l.SpanID,
		Time:   l.Time,
		Text:   append(l.Text, l2.Text...),
	}
}

func (l Line) IsComplete() bool {
	lineLen := len(l.Text)
	return lineLen > 0 && l.Text[lineLen-1] == newlineByte
}

func (l Line) Len() int {
	return len(l.Text)
}

func (l Line) String() string {
	return string(l.Text)
}

func linesFromBytes(spanID SpanID, time time.Time, bs []byte) []Line {
	lines := []Line{}
	lastBreak := 0
	for i, b := range bs {
		if b == newlineByte {
			lines = append(lines, Line{
				SpanID: spanID,
				Time:   time,
				Text:   bs[lastBreak : i+1],
			})
			lastBreak = i + 1
		}
	}
	if lastBreak < len(bs) {
		lines = append(lines, Line{
			SpanID: spanID,
			Time:   time,
			Text:   bs[lastBreak:],
		})
	}
	return lines
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
	spans map[SpanID]*Span
	lines []Line
	len   int
}

func NewLogStore() *LogStore {
	return &LogStore{
		spans: make(map[SpanID]*Span),
		lines: []Line{},
		len:   0,
	}
}

func (s *LogStore) Append(le LogEvent, secrets model.SecretSet) {
	// TODO(nick): This is a white lie until we have the code instrumented
	// to create real span ids.
	spanID := SpanID(le.Source())
	span, ok := s.spans[spanID]
	if !ok {
		span = &Span{ManifestName: le.Source(), LastLineIndex: -1}
		s.spans[spanID] = span
	}

	msg := secrets.Scrub(le.Message())

	isStartingNewLine := false
	if span.LastLineIndex == -1 {
		isStartingNewLine = true
	} else if s.lines[span.LastLineIndex].IsComplete() {
		isStartingNewLine = true
	}

	addedLines := linesFromBytes(spanID, le.Time(), msg)
	if len(addedLines) == 0 {
		return
	}

	if isStartingNewLine {
		s.lines = append(s.lines, addedLines...)
		span.LastLineIndex = len(s.lines) - 1
	} else {
		s.lines[span.LastLineIndex] = s.lines[span.LastLineIndex].Append(addedLines[0])
		s.lines = append(s.lines, addedLines[1:]...)

		if len(addedLines) > 1 {
			span.LastLineIndex = len(s.lines) - 1
		}
	}

	s.len += len(msg)
	s.ensureMaxLength()
}

// Get at most N lines from the tail of the log.
func (s *LogStore) Tail(n int) *LogStore {
	if len(s.lines) <= n {
		return s
	}

	newLines := s.lines[len(s.lines)-n:]
	newSpans := make(map[SpanID]*Span)
	for _, line := range newLines {
		newSpans[line.SpanID] = s.spans[line.SpanID].Clone()
	}
	return &LogStore{spans: newSpans, lines: newLines}
}

func (s *LogStore) String() string {
	sb := strings.Builder{}
	lastLine := Line{}
	for i, line := range s.lines {
		spanID := line.SpanID
		if i > 0 && lastLine.SpanID != line.SpanID && !lastLine.IsComplete() {
			// Insert a new line, so that we don't print two logs from different
			// spans on the same line.
			sb.WriteString("\n")
		}

		span := s.spans[spanID]
		if span.ManifestName != "" {
			sb.WriteString(fmt.Sprintf("%s | ", span.ManifestName))
		}
		sb.WriteString(string(line.Text))

		lastLine = line
	}
	return sb.String()
}

func (s *LogStore) computeLen() int {
	result := 0
	for _, line := range s.lines {
		result += line.Len()
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
	for i := len(s.lines) - 1; i >= 0; i-- {
		line := s.lines[i]
		bytesSpent += line.Len()
		if truncationIndex == -1 && bytesSpent > logTruncationTarget {
			truncationIndex = i + 1
		}
		if bytesSpent > maxLogLengthInBytes {
			s.lines = s.lines[truncationIndex:]
			for _, span := range s.spans {
				span.LastLineIndex -= truncationIndex
				if span.LastLineIndex < 0 {
					span.LastLineIndex = -1
				}
			}
			s.len = s.computeLen()
		}
	}
}
