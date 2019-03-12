package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// At this limit, with one resource having a 120k log, render time was ~20ms and CPU usage was ~70% on an MBP.
// 70% still isn't great when tilt doesn't really have any necessary work to do, but at least it's usable.
// A render time of ~40ms was about when the interface started being noticeably laggy to me.
const maxLogLengthInBytes = 120 * 1000

type Log struct {
	content []byte
}

func NewLog(s string) Log {
	return Log{[]byte(s)}
}

func (l Log) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(l.content))
}

func (l Log) String() string {
	return string(l.content)
}

func (l Log) Empty() bool {
	return len(l.content) == 0
}

func timestampPrefix(ts time.Time) []byte {
	t := ts.Format("2006/01/02 15:04:05")
	return []byte(fmt.Sprintf("%s ", t))
}

// Returns a new instance of `Log` with content equal to `b` appended to the end of `l`
// Performs truncation off the start of the log (at a newline) to ensure the resulting log is not
// longer than `maxLogLengthInBytes`. (which maybe means a pedant would say this isn't strictly an `append`?)
func AppendLog(l Log, le LogEvent, timestampsEnabled bool) Log {
	content := l.content

	// if we're starting a new line, we need a timestamp
	if len(l.content) > 0 && l.content[len(l.content)-1] == '\n' && timestampsEnabled {
		content = append(content, timestampPrefix(le.Time())...)
	}

	b := le.Message()

	if timestampsEnabled {
		b = addTimestamps(b, le.Time())
	}

	content = append(content, b...)

	content = ensureMaxLength(content)

	return Log{content}
}

type LogEvent interface {
	Message() []byte
	Time() time.Time
}

func addTimestamps(bs []byte, ts time.Time) []byte {
	// if the last char is a newline, temporarily remove it so that ReplaceAll doesn't get it
	// (we don't want "foo\n" to turn into "foo\nTIMESTAMP")
	endsInNewline := false
	if len(bs) > 0 && bs[len(bs)-1] == '\n' {
		endsInNewline = true
		bs = bs[:len(bs)-1]
	}

	nl := []byte("\n")
	p := append(nl, timestampPrefix(ts)...)
	ret := bytes.ReplaceAll(bs, nl, p)

	if endsInNewline {
		ret = append(ret, '\n')
	}
	return ret
}

func ensureMaxLength(b []byte) []byte {
	if len(b) > maxLogLengthInBytes {
		for i := len(b) - maxLogLengthInBytes - 1; i < len(b); i++ {
			if b[i] == '\n' {
				b = b[i+1:]
				break
			}
		}
	}

	return b
}
