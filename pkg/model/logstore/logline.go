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

type logLineBuilder struct {
	span        *Span
	segments    []LogSegment
	isFirstLine bool

	needsTrailingNewline bool
}

func newLogLineBuilder(span *Span, segment LogSegment, isFirstLine bool) *logLineBuilder {
	return &logLineBuilder{
		span:        span,
		segments:    []LogSegment{segment},
		isFirstLine: isFirstLine,
	}
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

func (b *logLineBuilder) build(options logOptions) []LogLine {
	result := []LogLine{}

	segment := b.segments[0]
	buildEvent := segment.Fields[logger.FieldNameBuildEvent]
	if buildEvent == "init" {
		result = append(result, b.buildSpaceLine(options))
	}

	result = append(result, b.buildMainLine(options))
	return result
}

func (b *logLineBuilder) buildSpaceLine(options logOptions) LogLine {
	sb := strings.Builder{}
	span := b.span
	segment := b.segments[0]
	spanID := segment.SpanID
	time := segment.Time
	if options.showManifestPrefix && span.ManifestName != "" {
		shouldSkip := options.skipFirstLineManifestPrefix && b.isFirstLine
		if !shouldSkip {
			sb.WriteString(SourcePrefix(span.ManifestName))
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

	sb := strings.Builder{}
	if options.showManifestPrefix && span.ManifestName != "" {
		shouldSkip := options.skipFirstLineManifestPrefix && b.isFirstLine
		if !shouldSkip {
			sb.WriteString(SourcePrefix(span.ManifestName))
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
