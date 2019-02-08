package model

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

func (l Log) String() string {
	return string(l.content)
}

func (l Log) Empty() bool {
	return len(l.content) == 0
}

func (l Log) Copy() Log {
	return Log{append([]byte{}, l.content...)}
}

func (l *Log) Append(b []byte) {
	l.content = append(l.content, b...)
	if len(l.content) > maxLogLengthInBytes {
		for i := len(l.content) - maxLogLengthInBytes - 1; i < len(l.content); i++ {
			if l.content[i] == '\n' {
				l.content = l.content[i+1:]
				break
			}
		}
	}
}
