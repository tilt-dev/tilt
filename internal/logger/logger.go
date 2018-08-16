package logger

import (
	"context"
	"fmt"
)

type Logger interface {
	Info(format string, a ...interface{})
	Verbose(format string, a ...interface{})
	Debug(format string, a ...interface{})
}

type Level int

const (
	_ = iota
	InfoLvl
	VerboseLvl
	DebugLvl
)

const loggerContextKey = "Logger"

func Get(ctx context.Context) Logger {
	return ctx.Value(loggerContextKey).(Logger)
}

func NewLogger(level Level) Logger {
	return logger{level}
}

func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

type logger struct {
	level Level
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
		fmt.Printf(format+"\n", a...)
	}
}
