package logger

import (
	"fmt"
	"io"
)

// A logger that writes all of its messages to `write`
type funcLogger struct {
	supportsColor bool
	level         Level
	write         func(level Level, b []byte) error
}

var _ Logger = funcLogger{}

func NewFuncLogger(supportsColor bool, level Level, write func(level Level, b []byte) error) Logger {
	return funcLogger{supportsColor, level, write}
}

func (l funcLogger) Level() Level {
	return l.level
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
	_ = l.write(level, bytes)
}

func (l funcLogger) WriteString(level Level, s string) {
	_ = l.write(level, []byte(s))
}

type FuncLoggerWriter struct {
	l     funcLogger
	level Level
}

var _ io.Writer = FuncLoggerWriter{}

func (fw FuncLoggerWriter) Write(b []byte) (int, error) {
	return len(b), fw.l.write(fw.level, b)
}

func (l funcLogger) Writer(level Level) io.Writer {
	return FuncLoggerWriter{l, level}
}

func (l funcLogger) SupportsColor() bool {
	return l.supportsColor
}

var _ Logger = funcLogger{}
