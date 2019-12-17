package logstore

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/webview"
)

// At this limit, with one resource having a 120k log, render time was ~20ms and CPU usage was ~70% on an MBP.
// 70% still isn't great when tilt doesn't really have any necessary work to do, but at least it's usable.
// A render time of ~40ms was about when the interface started being noticeably laggy to me.
const defaultMaxLogLengthInBytes = 120 * 1000

const newlineByte = byte('\n')

type Span struct {
	ManifestName      model.ManifestName
	LastSegmentIndex  int
	FirstSegmentIndex int
}

func (s *Span) Clone() *Span {
	clone := *s
	return &clone
}

type SpanID = model.LogSpanID

type LogSegment struct {
	SpanID SpanID
	Time   time.Time
	Text   []byte
	Level  logger.Level

	// Continues a line from a previous segment.
	ContinuesLine bool
}

// Whether these two log segments may be printed on the same line
func (l LogSegment) CanContinueLine(other LogSegment) bool {
	return l.SpanID == other.SpanID && l.Level == other.Level
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

func segmentsFromBytes(spanID SpanID, time time.Time, level logger.Level, bs []byte) []LogSegment {
	segments := []LogSegment{}
	lastBreak := 0
	for i, b := range bs {
		if b == newlineByte {
			segments = append(segments, LogSegment{
				SpanID: spanID,
				Level:  level,
				Time:   time,
				Text:   bs[lastBreak : i+1],
			})
			lastBreak = i + 1
		}
	}
	if lastBreak < len(bs) {
		segments = append(segments, LogSegment{
			SpanID: spanID,
			Level:  level,
			Time:   time,
			Text:   bs[lastBreak:],
		})
	}
	return segments
}

type LogEvent interface {
	Message() []byte
	Time() time.Time
	Level() logger.Level

	// The manifest that this log is associated with.
	ManifestName() model.ManifestName

	// The SpanID that identifies what Span this is associated with in the LogStore.
	SpanID() SpanID
}

// An abstract checkpoint in the log store, so we can
// ask questions like "give me all logs since checkpoint X" and
// "scrub everything since checkpoint Y". In practice, this
// is just an index into the segment slice.
type Checkpoint int

// A central place for storing logs. Not thread-safe.
//
// If you need to read logs in a thread-safe way outside of
// the normal Store state loop, take a look at logstore.Reader.
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

	// Used for truncating the log. Set as a property so that we can change it
	// for testing.
	maxLogLengthInBytes int

	// If the log is truncated, we need to adjust all checkpoints
	checkpointOffset Checkpoint
}

func NewLogStoreForTesting(msg string) *LogStore {
	s := NewLogStore()
	s.Append(newGlobalTestLogEvent(msg), nil)
	return s
}

func NewLogStore() *LogStore {
	return &LogStore{
		spans:               make(map[SpanID]*Span),
		segments:            []LogSegment{},
		len:                 0,
		maxLogLengthInBytes: defaultMaxLogLengthInBytes,
	}
}

func (s *LogStore) Checkpoint() Checkpoint {
	return s.checkpointFromIndex(len(s.segments))
}

func (s *LogStore) checkpointFromIndex(index int) Checkpoint {
	return Checkpoint(index) + s.checkpointOffset
}

func (s *LogStore) checkpointToIndex(c Checkpoint) int {
	index := int(c - s.checkpointOffset)
	if index < 0 {
		return 0
	}
	if index > len(s.segments) {
		return len(s.segments)
	}
	return index
}

func (s *LogStore) ScrubSecretsStartingAt(secrets model.SecretSet, checkpoint Checkpoint) {
	index := s.checkpointToIndex(checkpoint)
	for i := index; i < len(s.segments); i++ {
		s.segments[i].Text = secrets.Scrub(s.segments[i].Text)
	}

	s.len = s.computeLen()
}

func (s *LogStore) Append(le LogEvent, secrets model.SecretSet) {
	spanID := le.SpanID()
	if spanID == "" && le.ManifestName() != "" {
		spanID = SpanID(fmt.Sprintf("unknown:%s", le.ManifestName()))
	}
	span, ok := s.spans[spanID]
	if !ok {
		span = &Span{
			ManifestName:      le.ManifestName(),
			LastSegmentIndex:  -1,
			FirstSegmentIndex: len(s.segments),
		}
		s.spans[spanID] = span
	}

	msg := secrets.Scrub(le.Message())
	added := segmentsFromBytes(spanID, le.Time(), le.Level(), msg)
	if len(added) == 0 {
		return
	}

	added[0].ContinuesLine = s.computeContinuesLine(added[0], span)

	s.segments = append(s.segments, added...)
	span.LastSegmentIndex = len(s.segments) - 1

	s.len += len(msg)
	s.ensureMaxLength()
}

func (s *LogStore) Empty() bool {
	return len(s.segments) == 0
}

// Get at most N lines from the tail of the log.
func (s *LogStore) Tail(n int) string {
	if n <= 0 {
		return ""
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
		return s.String()
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

	tempStore := &LogStore{spans: s.cloneSpanMap(), segments: newSegments}
	tempStore.recomputeDerivedValues()
	return tempStore.String()
}

func (s *LogStore) cloneSpanMap() map[SpanID]*Span {
	newSpans := make(map[SpanID]*Span, len(s.spans))
	for spanID, span := range s.spans {
		newSpans[spanID] = span.Clone()
	}
	return newSpans
}

func (s *LogStore) computeContinuesLine(seg LogSegment, span *Span) bool {
	if span.LastSegmentIndex == -1 {
		return false
	} else {
		lastSeg := s.segments[span.LastSegmentIndex]
		if lastSeg.IsComplete() {
			return false
		}
		if !lastSeg.CanContinueLine(seg) {
			return false
		}
	}

	return true
}

func (s *LogStore) recomputeDerivedValues() {
	s.len = s.computeLen()

	// Reset the last segment index so we can rebuild them from scratch.
	for _, span := range s.spans {
		span.FirstSegmentIndex = -1
		span.LastSegmentIndex = -1
	}

	// Rebuild information about line continuations.
	for i, segment := range s.segments {
		spanID := segment.SpanID
		span := s.spans[spanID]
		if span.FirstSegmentIndex == -1 {
			span.FirstSegmentIndex = i
		}

		s.segments[i].ContinuesLine = s.computeContinuesLine(segment, span)
		span.LastSegmentIndex = i
	}

	for spanID, span := range s.spans {
		if span.FirstSegmentIndex == -1 {
			delete(s.spans, spanID)
		}
	}
}

// Returns logs incrementally from the given checkpoint.
//
// In many use cases, logs are printed to an append-only stream (like os.Stdout).
// Once they've been printed, they can't be called back.
// ContinuingString() tries to make reasonable product decisions about printing
// all the logs that have streamed in since the given checkpoint.
//
// Typical usage, looks like:
//
// Print(store.ContinuingString(state.LastCheckpoint))
// state.LastCheckpoint = store.Checkpoint()
func (s *LogStore) ContinuingString(checkpoint Checkpoint) string {
	isSameSpanContinuation := false
	isChangingSpanContinuation := false
	checkpointIndex := s.checkpointToIndex(checkpoint)
	precedingIndex := checkpointIndex - 1
	if precedingIndex >= 0 && checkpointIndex < len(s.segments) {
		// Check the last thing we printed. If it was wasn't complete,
		// we have to do some extra work to properly continue the previous print.
		precedingSegment := s.segments[precedingIndex]
		currentSegment := s.segments[checkpointIndex]
		if !precedingSegment.IsComplete() {
			// If this is the same span id, remove the prefix from this line.
			if precedingSegment.CanContinueLine(currentSegment) {
				isSameSpanContinuation = true
			} else {
				isChangingSpanContinuation = true
			}
		}
	}

	tempSegments := s.segments[checkpointIndex:]
	tempLogStore := &LogStore{
		spans:    s.cloneSpanMap(),
		segments: tempSegments,
	}
	tempLogStore.recomputeDerivedValues()

	if isSameSpanContinuation {
		spanID := tempSegments[0].SpanID
		span := s.spans[spanID]
		return strings.TrimPrefix(tempLogStore.String(), SourcePrefix(span.ManifestName))
	}
	if isChangingSpanContinuation {
		return "\n" + tempLogStore.String()
	}
	return tempLogStore.String()
}

func (s *LogStore) ToLogList(fromCheckpoint Checkpoint) (*webview.LogList, error) {
	spans := make(map[string]*webview.LogSpan, len(s.spans))
	for spanID, span := range s.spans {
		spans[string(spanID)] = &webview.LogSpan{
			ManifestName: span.ManifestName.String(),
		}
	}

	startIndex := s.checkpointToIndex(fromCheckpoint)
	if startIndex >= len(s.segments) {
		// No logs to send down.
		return &webview.LogList{
			FromCheckpoint: -1,
			ToCheckpoint:   -1,
		}, nil
	}

	segments := make([]*webview.LogSegment, 0, len(s.segments)-startIndex)
	for i := startIndex; i < len(s.segments); i++ {
		segment := s.segments[i]
		time, err := ptypes.TimestampProto(segment.Time)
		if err != nil {
			return nil, errors.Wrap(err, "ToLogList")
		}
		segments = append(segments, &webview.LogSegment{
			SpanId: string(segment.SpanID),
			Level:  webview.LogLevel(segment.Level),
			Time:   time,
			Text:   string(segment.Text),
		})
	}

	return &webview.LogList{
		Spans:          spans,
		Segments:       segments,
		FromCheckpoint: int32(s.checkpointFromIndex(startIndex)),
		ToCheckpoint:   int32(s.Checkpoint()),
	}, nil
}

func (s *LogStore) String() string {
	return s.logHelper(s.spans, true)
}

func (s *LogStore) spansForManifest(mn model.ManifestName) map[SpanID]*Span {
	result := make(map[SpanID]*Span)
	for spanID, span := range s.spans {
		if span.ManifestName == mn {
			result[spanID] = span
		}
	}
	return result
}

func (s *LogStore) SpanLog(spanID SpanID) string {
	spans := make(map[SpanID]*Span)
	span, ok := s.spans[spanID]
	if !ok {
		return ""
	}
	spans[spanID] = span
	return s.logHelper(spans, false)
}

func (s *LogStore) ManifestLog(mn model.ManifestName) string {
	spans := s.spansForManifest(mn)
	return s.logHelper(spans, false)
}

func (s *LogStore) logHelper(spansToLog map[SpanID]*Span, showManifestPrefix bool) string {
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
	earliestStartIndex := -1
	latestEndIndex := -1
	for _, span := range spansToLog {
		if earliestStartIndex == -1 || span.FirstSegmentIndex < earliestStartIndex {
			earliestStartIndex = span.FirstSegmentIndex
		}
		if latestEndIndex == -1 || span.LastSegmentIndex > latestEndIndex {
			latestEndIndex = span.LastSegmentIndex
		}
	}

	if earliestStartIndex == -1 {
		return ""
	}

	startIndex := earliestStartIndex
	lastIndex := latestEndIndex

	isFirstLine := true
	for i := startIndex; i <= lastIndex; i++ {
		segment := s.segments[i]
		if !segment.StartsLine() {
			continue
		}

		spanID := segment.SpanID
		span := s.spans[spanID]
		if _, ok := spansToLog[spanID]; !ok {
			continue
		}

		// If the last segment never completed, print a newline now, so that the
		// logs from different sources don't blend together.
		if !isFirstLine && !lastLineCompleted {
			sb.WriteString("\n")
		}

		if showManifestPrefix && span.ManifestName != "" {
			sb.WriteString(SourcePrefix(span.ManifestName))
		}
		sb.WriteString(string(segment.Text))
		isFirstLine = false

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

			if !currentSeg.CanContinueLine(segment) {
				break
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

// After a log hits its limit, we need to truncate it to keep it small
// we do this by cutting a big chunk at a time, so that we have rarer, larger changes, instead of
// a small change every time new data is written to the log
// https://github.com/windmilleng/tilt/issues/1935#issuecomment-531390353
func (s *LogStore) logTruncationTarget() int {
	return s.maxLogLengthInBytes / 2
}

func (s *LogStore) ensureMaxLength() {
	if s.len <= s.maxLogLengthInBytes {
		return
	}

	// Figure out where we have to truncate.
	bytesSpent := 0
	truncationIndex := -1
	for i := len(s.segments) - 1; i >= 0; i-- {
		segment := s.segments[i]
		bytesSpent += segment.Len()
		if truncationIndex == -1 && bytesSpent > s.logTruncationTarget() {
			truncationIndex = i + 1
		}
		if bytesSpent > s.maxLogLengthInBytes {
			s.segments = s.segments[truncationIndex:]
			s.checkpointOffset += Checkpoint(truncationIndex)
			s.recomputeDerivedValues()
			return
		}
	}
}
