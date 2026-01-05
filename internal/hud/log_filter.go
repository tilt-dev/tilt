package hud

import (
	"fmt"
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

// FilterSince represents a duration for time-based log filtering.
// Zero value means no time filter.
type FilterSince time.Duration

// FilterTail represents the number of lines to show from the end.
// -1 means no limit, 0+ means limit to that many lines.
type FilterTail int

// FilterJSON indicates whether to output logs in JSON format.
type FilterJSON bool

// FilterJSONFields specifies which fields to include in JSON output.
// Empty string means default (minimal), "full" means all fields,
// or comma-separated field names.
type FilterJSONFields string

// JSONFieldSet represents the set of fields to include in JSON output.
type JSONFieldSet struct {
	Time       bool
	Resource   bool
	Level      bool
	Message    bool
	SpanID     bool
	ProgressID bool
	BuildEvent bool
	Source     bool
}

// MinimalJSONFields returns the default minimal field set.
func MinimalJSONFields() JSONFieldSet {
	return JSONFieldSet{
		Time:     true,
		Resource: true,
		Level:    true,
		Message:  true,
	}
}

// FullJSONFields returns all available fields.
func FullJSONFields() JSONFieldSet {
	return JSONFieldSet{
		Time:       true,
		Resource:   true,
		Level:      true,
		Message:    true,
		SpanID:     true,
		ProgressID: true,
		BuildEvent: true,
		Source:     true,
	}
}

// ValidJSONFieldNames lists all valid field names for --json-fields.
var ValidJSONFieldNames = []string{"time", "resource", "level", "message", "spanid", "progressid", "buildevent", "source", "minimal", "full"}

// ParseJSONFields parses the --json-fields flag value into a JSONFieldSet.
// Returns an error if unknown field names are provided.
func ParseJSONFields(s string) (JSONFieldSet, error) {
	if s == "" || s == "minimal" {
		return MinimalJSONFields(), nil
	}
	if s == "full" {
		return FullJSONFields(), nil
	}

	// Parse comma-separated field names
	// Scan all fields first to detect unknowns before applying presets
	result := JSONFieldSet{}
	var unknownFields []string
	hasFull := false
	for _, field := range strings.Split(s, ",") {
		field = strings.TrimSpace(field)
		switch strings.ToLower(field) {
		case "time":
			result.Time = true
		case "resource":
			result.Resource = true
		case "level":
			result.Level = true
		case "message":
			result.Message = true
		case "spanid":
			result.SpanID = true
		case "progressid":
			result.ProgressID = true
		case "buildevent":
			result.BuildEvent = true
		case "source":
			result.Source = true
		case "minimal":
			// Include minimal preset fields
			result.Time = true
			result.Resource = true
			result.Level = true
			result.Message = true
		case "full":
			// Mark for full fields, but continue scanning for unknowns
			hasFull = true
		default:
			if field != "" {
				unknownFields = append(unknownFields, field)
			}
		}
	}

	if len(unknownFields) > 0 {
		return JSONFieldSet{}, fmt.Errorf("unknown --json-fields: %s (valid: %s)",
			strings.Join(unknownFields, ", "),
			strings.Join(ValidJSONFieldNames, ", "))
	}

	if hasFull {
		return FullJSONFields(), nil
	}

	return result, nil
}

func NewLogFilter(
	source FilterSource,
	resources FilterResources,
	level FilterLevel,
	since FilterSince,
	tail FilterTail,
	jsonOutput FilterJSON,
	jsonFields FilterJSONFields,
) (LogFilter, error) {
	fields, err := ParseJSONFields(string(jsonFields))
	if err != nil {
		return LogFilter{}, err
	}
	return LogFilter{
		source:     source,
		resources:  resources,
		level:      logger.Level(level),
		since:      time.Duration(since),
		tail:       int(tail),
		jsonOutput: bool(jsonOutput),
		jsonFields: fields,
	}, nil
}

type LogFilter struct {
	source     FilterSource
	resources  FilterResources
	level      logger.Level
	since      time.Duration
	tail       int // -1 means no limit
	jsonOutput bool
	jsonFields JSONFieldSet
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

// if printing logs for only one resource, don't need resource name prefix
func (f LogFilter) SuppressPrefix() bool {
	return len(f.resources) == 1
}

// matchesSinceFilter checks if the log line is within the since time window.
func (f LogFilter) matchesSinceFilter(line logstore.LogLine, now time.Time) bool {
	if f.since == 0 {
		return true // no time filter
	}
	cutoff := now.Add(-f.since)
	return !line.Time.Before(cutoff)
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

// MatchesWithTime checks if this line matches the filter including time-based filtering.
func (f LogFilter) MatchesWithTime(line logstore.LogLine, now time.Time) bool {
	if !f.Matches(line) {
		return false
	}
	return f.matchesSinceFilter(line, now)
}

func (f LogFilter) Apply(lines []logstore.LogLine) []logstore.LogLine {
	return f.ApplyWithOptions(lines, time.Now(), true)
}

// ApplyWithTime applies the filter with a specific time for testing.
func (f LogFilter) ApplyWithTime(lines []logstore.LogLine, now time.Time) []logstore.LogLine {
	return f.ApplyWithOptions(lines, now, true)
}

// ApplyWithoutTail applies the filter but skips the tail limit.
// Use this for streaming batches after the initial history.
func (f LogFilter) ApplyWithoutTail(lines []logstore.LogLine) []logstore.LogLine {
	return f.ApplyWithOptions(lines, time.Now(), false)
}

// ApplyWithOptions applies the filter with control over tail application.
func (f LogFilter) ApplyWithOptions(lines []logstore.LogLine, now time.Time, applyTail bool) []logstore.LogLine {
	filtered := []logstore.LogLine{}
	for _, line := range lines {
		if f.MatchesWithTime(line, now) {
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

// JSONFields returns the JSON field configuration.
func (f LogFilter) JSONFields() JSONFieldSet {
	return f.jsonFields
}

// Tail returns the tail limit (-1 for no limit).
func (f LogFilter) Tail() int {
	return f.tail
}
