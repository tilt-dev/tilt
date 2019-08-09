package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// At this limit, with one resource having a 120k log, render time was ~20ms and CPU usage was ~70% on an MBP.
// 70% still isn't great when tilt doesn't really have any necessary work to do, but at least it's usable.
// A render time of ~40ms was about when the interface started being noticeably laggy to me.
const maxLogLengthInBytes = 120 * 1000

const newlineByte = byte('\n')

// All LogLines should end in a \n to be considered "complete".
// We expect this will have more metadata over time about where the line came from.
type logLine []byte

func (l logLine) IsComplete() bool {
	lineLen := len(l)
	return lineLen > 0 && l[lineLen-1] == newlineByte
}

func (l logLine) Len() int {
	return len(l)
}

func (l logLine) String() string {
	return string(l)
}

func linesFromString(s string) []logLine {
	return linesFromBytes([]byte(s))
}

func linesFromBytes(bs []byte) []logLine {
	lines := []logLine{}
	lastBreak := 0
	for i, b := range bs {
		if b == newlineByte {
			lines = append(lines, bs[lastBreak:i+1])
			lastBreak = i + 1
		}
	}
	if lastBreak < len(bs) {
		lines = append(lines, bs[lastBreak:])
	}
	return lines
}

type Log struct {
	lines []logLine
}

func NewLog(s string) Log {
	return Log{lines: linesFromString(s)}
}

// Get at most N lines from the tail of the log.
func (l Log) Tail(n int) Log {
	if len(l.lines) <= n {
		return l
	}
	return Log{lines: l.lines[len(l.lines)-n:]}
}

func (l Log) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

func (l Log) Len() int {
	result := 0
	for _, line := range l.lines {
		result += len(line)
	}
	return result
}

func (l Log) String() string {
	lines := make([]string, len(l.lines))
	for i, line := range l.lines {
		lines[i] = line.String()
	}
	return strings.Join(lines, "")
}

func (l Log) Empty() bool {
	return l.Len() == 0
}

func TimestampPrefix(ts time.Time) []byte {
	t := ts.Format("2006/01/02 15:04:05")
	return []byte(fmt.Sprintf("%s ", t))
}

// Returns a new instance of `Log` with content equal to `b` appended to the end of `l`
// Performs truncation off the start of the log (at a newline) to ensure the resulting log is not
// longer than `maxLogLengthInBytes`. (which maybe means a pedant would say this isn't strictly an `append`?)
func AppendLog(l Log, le LogEvent, timestampsEnabled bool, prefix string) Log {
	isStartingNewLine := len(l.lines) == 0 || l.lines[len(l.lines)-1].IsComplete()
	addedLines := linesFromBytes(le.Message())
	if len(addedLines) == 0 {
		return l
	}

	if timestampsEnabled {
		ts := le.Time()
		for i, line := range addedLines {
			if i != 0 || isStartingNewLine {
				addedLines[i] = append(TimestampPrefix(ts), line...)
			}
		}
	}

	if len(prefix) > 0 {
		for i, line := range addedLines {
			if i != 0 || isStartingNewLine {
				addedLines[i] = append([]byte(prefix), line...)
			}
		}
	}

	var newLines []logLine
	if isStartingNewLine {
		newLines = append(l.lines, addedLines...)
	} else {
		lastIndex := len(l.lines) - 1
		newLastLine := append(l.lines[lastIndex], addedLines[0]...)

		// We have to be a bit careful here to avoid mutating the original Log struct.
		newLines = append(l.lines[0:lastIndex], newLastLine)
		newLines = append(newLines, addedLines[1:]...)
	}

	return Log{ensureMaxLength(newLines)}
}

type LogEvent interface {
	Message() []byte
	Time() time.Time
}

func ensureMaxLength(lines []logLine) []logLine {
	bytesLeft := maxLogLengthInBytes
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if line.Len() > bytesLeft {
			return lines[i+1:]
		}
		bytesLeft -= line.Len()
	}

	return lines
}
