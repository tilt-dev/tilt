package logstore

import (
	"strings"
	"time"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type LogLine struct {
	Text         string
	SpanID       SpanID
	ProgressID   string
	Level        logger.Level
	BuildEvent   string
	ManifestName model.ManifestName

	// Most progress lines are optional. For example, if a bunch
	// of little upload updates come in, it's ok to skip some.
	//
	// ProgressMustPrint indicates that this line must appear in the
	// output - e.g., a line that communicates that the upload finished.
	ProgressMustPrint bool

	Time time.Time
}

// Accumulates the segments of one line. A single builder is reused across
// all lines of a toLogLines call (start/reset recycle the segment buffer),
// so it must not retain segment state past reset.
type logLineBuilder struct {
	span        *Span
	segments    []LogSegment
	isFirstLine bool

	needsTrailingNewline bool
}

// Begins a new line, reusing the segment buffer of any previously consumed
// line.
func (b *logLineBuilder) start(span *Span, segment LogSegment, isFirstLine bool) {
	b.span = span
	b.segments = append(b.segments[:0], segment)
	b.isFirstLine = isFirstLine
	b.needsTrailingNewline = false
}

// An active builder holds at least the segment that started its line.
func (b *logLineBuilder) active() bool {
	return len(b.segments) > 0
}

func (b *logLineBuilder) reset() {
	b.span = nil
	b.segments = b.segments[:0]
	b.needsTrailingNewline = false
}

func (b *logLineBuilder) addSegment(segment LogSegment) {
	b.segments = append(b.segments, segment)
}

func (b *logLineBuilder) lastSegment() LogSegment {
	return b.segments[len(b.segments)-1]
}

func (b *logLineBuilder) isComplete() bool {
	return b.lastSegment().IsComplete()
}

// Appends the built line(s) onto result rather than returning a fresh
// slice, so the per-line cost is only the line text itself.
func (b *logLineBuilder) appendTo(result []LogLine, options logOptions) []LogLine {
	segment := b.segments[0]
	buildEvent := segment.Fields[logger.FieldNameBuildEvent]
	if buildEvent == "init" {
		result = append(result, b.buildSpaceLine(options))
	}

	return append(result, b.buildMainLine(options))
}

func (b *logLineBuilder) buildSpaceLine(options logOptions) LogLine {
	sb := strings.Builder{}
	sb.Grow(sourcePrefixReserveLen + 1)
	span := b.span
	segment := b.segments[0]
	spanID := segment.SpanID
	time := segment.Time
	if options.showManifestPrefix && span.ManifestName != "" {
		shouldSkip := options.skipFirstLineManifestPrefix && b.isFirstLine
		if !shouldSkip {
			appendSourcePrefix(&sb, span.ManifestName)
		}
	}
	sb.WriteString("\n")

	return LogLine{
		Text:         sb.String(),
		SpanID:       spanID,
		Level:        segment.Level,
		BuildEvent:   segment.Fields[logger.FieldNameBuildEvent],
		ManifestName: span.ManifestName,
		Time:         time,
	}
}

func (b *logLineBuilder) buildMainLine(options logOptions) LogLine {
	segment := b.segments[0]
	span := b.span
	spanID := segment.SpanID
	time := segment.Time
	progressID := segment.Fields[logger.FieldNameProgressID]
	progressMustPrint := segment.Fields[logger.FieldNameProgressMustPrint] == "1"

	// Reserve for the worst case (prefix + anchor marker + text + trailing
	// newline) so each line's text is built in a single allocation.
	textLen := 0
	for _, seg := range b.segments {
		textLen += len(seg.Text)
	}
	sb := strings.Builder{}
	sb.Grow(textLen + sourcePrefixReserveLen + len("WARNING: ") + 1)

	if options.showManifestPrefix && span.ManifestName != "" {
		shouldSkip := options.skipFirstLineManifestPrefix && b.isFirstLine
		if !shouldSkip {
			appendSourcePrefix(&sb, span.ManifestName)
		}
	}

	if segment.Anchor {
		// TODO(nick): Add Terminal colors when supported.
		if segment.Level == logger.WarnLvl {
			sb.WriteString("WARNING: ")
		} else if segment.Level == logger.ErrorLvl {
			sb.WriteString("ERROR: ")
		}
	}

	for _, segment := range b.segments {
		sb.Write(segment.Text)
	}

	if !b.isComplete() && b.needsTrailingNewline {
		sb.WriteString("\n")
	}

	return LogLine{
		Text:              sb.String(),
		SpanID:            spanID,
		Level:             segment.Level,
		BuildEvent:        segment.Fields[logger.FieldNameBuildEvent],
		ManifestName:      span.ManifestName,
		ProgressID:        progressID,
		ProgressMustPrint: progressMustPrint,
		Time:              time,
	}
}
