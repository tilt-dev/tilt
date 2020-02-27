package logger

import (
	"fmt"
	"io"
)

// A logger that writes all of its messages to `write`
type funcLogger struct {
	supportsColor bool
	level         Level
	write         func(level Level, fields Fields, b []byte) error
	fields        Fields
}

var _ Logger = funcLogger{}

func NewFuncLogger(supportsColor bool, level Level, write func(level Level, fields Fields, b []byte) error) Logger {
	return funcLogger{
		supportsColor: supportsColor,
		level:         level,
		write:         write,
		fields:        nil,
	}
}

func (l funcLogger) WithFields(fields Fields) Logger {
	if len(fields) == 0 {
		return l
	}
	newFields := make(map[string]string, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	return funcLogger{
		supportsColor: l.supportsColor,
		level:         l.level,
		write:         l.write,
		fields:        newFields,
	}
}

func (l funcLogger) Level() Level {
	return l.level
}

func (l funcLogger) Warnf(format string, a ...interface{}) {
	l.WriteString(WarnLvl, fmt.Sprintf(format+"\n", a...))
}

func (l funcLogger) Errorf(format string, a ...interface{}) {
	l.WriteString(ErrorLvl, fmt.Sprintf(format+"\n", a...))
}

func (l funcLogger) Infof(format string, a ...interface{}) {
	l.WriteString(InfoLvl, fmt.Sprintf(format+"\n", a...))
}

func (l funcLogger) Verbosef(format string, a ...interface{}) {
	l.WriteString(VerboseLvl, fmt.Sprintf(format+"\n", a...))
}

func (l funcLogger) Debugf(format string, a ...interface{}) {
	l.WriteString(DebugLvl, fmt.Sprintf(format+"\n", a...))
}

func (l funcLogger) Write(level Level, bytes []byte) {
	if l.level.ShouldDisplay(level) {
		_ = l.write(level, l.fields, bytes)
	}
}

func (l funcLogger) WriteString(level Level, s string) {
	if l.level.ShouldDisplay(level) {
		_ = l.write(level, l.fields, []byte(s))
	}
}

type FuncLoggerWriter struct {
	l     funcLogger
	level Level
}

var _ io.Writer = FuncLoggerWriter{}

func (fw FuncLoggerWriter) Write(b []byte) (int, error) {
	fw.l.Write(fw.level, b)
	return len(b), nil
}

func (l funcLogger) Writer(level Level) io.Writer {
	return FuncLoggerWriter{l, level}
}

func (l funcLogger) SupportsColor() bool {
	return l.supportsColor
}

var _ Logger = funcLogger{}
