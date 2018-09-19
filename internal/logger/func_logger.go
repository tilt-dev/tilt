package logger

import (
	"fmt"
	"io"
)

// A logger that writes all of its messages to `write`
type funcLogger struct {
	supportsColor bool
	write         func(level Level, b []byte) error
}

func NewFuncLogger(supportsColor bool, write func(level Level, b []byte) error) Logger {
	return funcLogger{supportsColor, write}
}

func (l funcLogger) Infof(format string, a ...interface{}) {
	l.Write(InfoLvl, fmt.Sprintf(format, a...))
}

func (l funcLogger) Verbosef(format string, a ...interface{}) {
	l.Write(VerboseLvl, fmt.Sprintf(format, a...))
}

func (l funcLogger) Debugf(format string, a ...interface{}) {
	l.Write(DebugLvl, fmt.Sprintf(format, a...))
}

func (l funcLogger) Write(level Level, s string) {
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
