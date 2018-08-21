package logger

import (
	"context"
	"fmt"
	"io"
)

type Logger interface {
	// log information that we always want to show
	Info(format string, a ...interface{})
	// log information that a tilt user might not want to see on every run, but that they might find
	// useful when debugging their Tiltfile/docker/k8s configs
	Verbose(format string, a ...interface{})
	// log information that is likely to only be of interest to tilt developers
	Debug(format string, a ...interface{})

	// gets an io.Writer that filters to the specified level for, e.g., passing to a subprocess
	Writer(level Level) io.Writer
}

var _ Logger = logger{}

type Level int

const (
	_ = iota
	InfoLvl
	VerboseLvl
	DebugLvl
)

const loggerContextKey = "Logger"

func Get(ctx context.Context) Logger {
	val := ctx.Value(loggerContextKey)

	if val != nil {
		return val.(Logger)
	}

	// No logger found in context, something is wrong.
	panic("Called logger.Get(ctx) on a context with no logger attached!")
}

func NewLogger(level Level, writer io.Writer) Logger {
	return logger{level, writer}
}

func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

type logger struct {
	level  Level
	writer io.Writer
}

func (l logger) Info(format string, a ...interface{}) {
	l.write(InfoLvl, format, a...)
}

func (l logger) Verbose(format string, a ...interface{}) {
	l.write(VerboseLvl, format, a...)
}

func (l logger) Debug(format string, a ...interface{}) {
	l.write(DebugLvl, format, a...)
}

func (l logger) write(level Level, format string, a ...interface{}) {
	if l.level >= level {
		// swallowing errors because:
		// 1) if we can't write to the log, what else are we going to do?
		// 2) a logger interface that returns error becomes really distracting at call sites,
		//    increasing friction and reducing logging
		_, _ = fmt.Fprintf(l.writer, format, a...)
		_, _ = fmt.Fprintln(l.writer, "")
	}
}

type levelWriter struct {
	logger logger
	level  Level
}

var _ io.Writer = levelWriter{}

func (lw levelWriter) Write(p []byte) (n int, err error) {
	if lw.logger.level >= lw.level {
		return lw.logger.writer.Write(p)
	} else {
		return len(p), nil
	}
}

func (l logger) Writer(level Level) io.Writer {
	return levelWriter{l, level}
}
