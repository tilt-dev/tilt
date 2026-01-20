package client

import (
	"strings"
	"time"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type FilterSource string

const (
	FilterSourceAll     FilterSource = "all"
	FilterSourceBuild   FilterSource = "build"
	FilterSourceRuntime FilterSource = "runtime"
)

func (s FilterSource) String() string { return string(s) }

type FilterResources []model.ManifestName

func (r FilterResources) Matches(name model.ManifestName) bool {
	if len(r) == 0 {
		return true
	}

	for _, n := range r {
		if n == name {
			return true
		}
	}

	return false
}

type FilterLevel logger.Level

// FilterSince represents an absolute timestamp for time-based log filtering.
// Zero value (time.Time{}) means no time filter.
// The CLI layer converts duration flags (e.g., "5m") to timestamps.
type FilterSince time.Time

// FilterTail represents the number of lines to show from the end.
// -1 means no limit, 0+ means limit to that many lines.
type FilterTail int

// FilterJSON indicates whether to output logs in JSON format.
type FilterJSON bool

func NewLogFilter(
	source FilterSource,
	resources FilterResources,
	level FilterLevel,
	since FilterSince,
	tail FilterTail,
	jsonOutput FilterJSON,
) LogFilter {
	return LogFilter{
		source:     source,
		resources:  resources,
		level:      logger.Level(level),
		since:      time.Time(since),
		tail:       int(tail),
		jsonOutput: bool(jsonOutput),
	}
}

type LogFilter struct {
	source     FilterSource
	resources  FilterResources
	level      logger.Level
	since      time.Time // zero value means no filter
	tail       int       // -1 means no limit
	jsonOutput bool
}

// The implementation is identical to isBuildSpanId in web/src/logs.ts.
func isBuildSpanID(spanID logstore.SpanID) bool {
	return strings.HasPrefix(string(spanID), "build:") || strings.HasPrefix(string(spanID), "cmdimage:")
}

// The implementation is identical to matchesLevelFilter in web/src/OverviewLogPane.tsx
func (f LogFilter) matchesLevelFilter(line logstore.LogLine) bool {
	if !f.level.AsSevereAs(logger.WarnLvl) {
		return true
	}

	return f.level == line.Level
}

// don't need the resource name prefix if:
// - printing logs for only one resource
// - printing logs to json
func (f LogFilter) SuppressPrefix() bool {
	return len(f.resources) == 1 || f.jsonOutput
}

// matchesSinceFilter checks if the log line is at or after the since timestamp.
func (f LogFilter) matchesSinceFilter(line logstore.LogLine) bool {
	if f.since.IsZero() {
		return true // no time filter
	}
	return !line.Time.Before(f.since)
}

// Matches Checks if this line matches the current filter.
// The implementation is identical to matchesFilter in web/src/OverviewLogPane.tsx.
// except for term filtering as tools like grep can be used from the CLI.
func (f LogFilter) Matches(line logstore.LogLine) bool {
	if line.BuildEvent != "" {
		// Always leave in build event logs.
		// This makes it easier to see which logs belong to which builds.
		return true
	}

	if !f.resources.Matches(line.ManifestName) {
		return false
	}

	isBuild := isBuildSpanID(line.SpanID)
	if f.source == FilterSourceRuntime && isBuild {
		return false
	}

	if f.source == FilterSourceBuild && !isBuild {
		return false
	}

	return f.matchesLevelFilter(line)
}

// MatchesAll checks if this line matches all filters including time-based filtering.
func (f LogFilter) MatchesAll(line logstore.LogLine) bool {
	if !f.Matches(line) {
		return false
	}
	return f.matchesSinceFilter(line)
}

func (f LogFilter) Apply(lines []logstore.LogLine) []logstore.LogLine {
	return f.ApplyWithOptions(lines, true)
}

// ApplyWithoutTail applies the filter but skips the tail limit.
// Use this for streaming batches after the initial history.
func (f LogFilter) ApplyWithoutTail(lines []logstore.LogLine) []logstore.LogLine {
	return f.ApplyWithOptions(lines, false)
}

// ApplyWithOptions applies the filter with control over tail application.
func (f LogFilter) ApplyWithOptions(lines []logstore.LogLine, applyTail bool) []logstore.LogLine {
	filtered := []logstore.LogLine{}
	for _, line := range lines {
		if f.MatchesAll(line) {
			filtered = append(filtered, line)
		}
	}

	// Apply tail filter after other filters, only if requested
	if applyTail && f.tail >= 0 && len(filtered) > f.tail {
		filtered = filtered[len(filtered)-f.tail:]
	}

	return filtered
}

// JSONOutput returns whether JSON output is enabled.
func (f LogFilter) JSONOutput() bool {
	return f.jsonOutput
}

// Tail returns the tail limit (-1 for no limit).
func (f LogFilter) Tail() int {
	return f.tail
}
