package model

import "encoding/json"

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

// Returns a new instance of `Log` with content equal to `b` appended to the end of `l`
// Performs truncation off the start of the log (at a newline) to ensure the resulting log is not
// longer than `maxLogLengthInBytes`. (which maybe means a pedant would say this isn't strictly an `append`?)
func AppendLog(l Log, b []byte) Log {
	content := append(l.content, b...)
	if len(content) > maxLogLengthInBytes {
		for i := len(content) - maxLogLengthInBytes - 1; i < len(content); i++ {
			if content[i] == '\n' {
				content = content[i+1:]
				break
			}
		}
	}

	return Log{content}
}
