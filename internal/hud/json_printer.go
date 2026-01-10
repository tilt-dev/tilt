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
}

// NewJSONPrinter creates a new JSON printer.
func NewJSONPrinter(stdout Stdout) *JSONPrinter {
	return &JSONPrinter{
		stdout: stdout,
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
	source := "runtime"
	if isBuildSpanID(line.SpanID) {
		source = "build"
	}

	return JSONLogLine{
		Time:       stringPtr(line.Time.Format(time.RFC3339)),
		Resource:   stringPtr(string(line.ManifestName)),
		Level:      stringPtr(levelToString(line.Level)),
		Message:    stringPtr(strings.TrimSuffix(line.Text, "\n")),
		SpanID:     stringPtr(string(line.SpanID)),
		ProgressID: stringPtr(line.ProgressID),
		BuildEvent: stringPtr(line.BuildEvent),
		Source:     stringPtr(source),
	}
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
