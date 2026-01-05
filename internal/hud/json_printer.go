package hud

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

// JSONLogLine represents a log line in JSON format.
// Fields use pointer types so we can distinguish between "not included" (nil)
// and "included but empty" (pointer to empty string).
type JSONLogLine struct {
	Time       *string `json:"time,omitempty"`
	Resource   *string `json:"resource,omitempty"`
	Level      *string `json:"level,omitempty"`
	Message    *string `json:"message,omitempty"`
	SpanID     *string `json:"spanID,omitempty"`
	ProgressID *string `json:"progressID,omitempty"`
	BuildEvent *string `json:"buildEvent,omitempty"`
	Source     *string `json:"source,omitempty"`
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string {
	return &s
}

// JSONPrinter outputs log lines as JSON Lines (one JSON object per line).
type JSONPrinter struct {
	stdout Stdout
	fields JSONFieldSet
}

// NewJSONPrinter creates a new JSON printer with the specified field set.
func NewJSONPrinter(stdout Stdout, fields JSONFieldSet) *JSONPrinter {
	return &JSONPrinter{
		stdout: stdout,
		fields: fields,
	}
}

// PrintNewline writes a newline to output.
func (p *JSONPrinter) PrintNewline() {
	_, _ = io.WriteString(p.stdout, "\n")
}

// Print outputs log lines as JSON Lines.
func (p *JSONPrinter) Print(lines []logstore.LogLine) {
	encoder := json.NewEncoder(p.stdout)
	for _, line := range lines {
		jsonLine := p.toJSONLine(line)
		_ = encoder.Encode(jsonLine)
	}
}

func (p *JSONPrinter) toJSONLine(line logstore.LogLine) JSONLogLine {
	result := JSONLogLine{}

	if p.fields.Time {
		result.Time = stringPtr(line.Time.Format(time.RFC3339))
	}
	if p.fields.Resource {
		result.Resource = stringPtr(string(line.ManifestName))
	}
	if p.fields.Level {
		result.Level = stringPtr(levelToString(line.Level))
	}
	if p.fields.Message {
		// Strip trailing newline from message for cleaner JSON
		result.Message = stringPtr(strings.TrimSuffix(line.Text, "\n"))
	}
	if p.fields.SpanID {
		result.SpanID = stringPtr(string(line.SpanID))
	}
	if p.fields.ProgressID {
		result.ProgressID = stringPtr(line.ProgressID)
	}
	if p.fields.BuildEvent {
		result.BuildEvent = stringPtr(line.BuildEvent)
	}
	if p.fields.Source {
		if isBuildSpanID(line.SpanID) {
			result.Source = stringPtr("build")
		} else {
			result.Source = stringPtr("runtime")
		}
	}

	return result
}

func levelToString(level logger.Level) string {
	switch level {
	case logger.DebugLvl:
		return "debug"
	case logger.VerboseLvl:
		return "verbose"
	case logger.InfoLvl:
		return "info"
	case logger.WarnLvl:
		return "warn"
	case logger.ErrorLvl:
		return "error"
	default:
		return "info"
	}
}
