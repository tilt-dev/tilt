package logger

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"

	"github.com/mattn/go-isatty"
)

// Logger with better controls for levels and colors.
//
// Note that our loggers often serve as both traditional loggers (where each
// call to PodLog() is a discrete log entry that may be emitted as JSON or with
// newlines) and as Writers (where each call to Write() may be part of a larger
// output stream, and each message may not end in a newline).
//
// Logger implementations that bridge these two worlds should have discrete
// messages (like Infof) append a newline to the string before passing it to
// Write().
type Logger interface {
	// log information that we always want to show
	Infof(format string, a ...interface{})
	// log information that a tilt user might not want to see on every run, but that they might find
	// useful when debugging their Tiltfile/docker/k8s configs
	Verbosef(format string, a ...interface{})
	// log information that is likely to only be of interest to tilt developers
	Debugf(format string, a ...interface{})

	Write(level Level, s string)

	// gets an io.Writer that filters to the specified level for, e.g., passing to a subprocess
	Writer(level Level) io.Writer

	Level() Level

	SupportsColor() bool
}

var _ Logger = logger{}

type Level int

const (
	NoneLvl = iota
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
	// adapted from fatih/color
	supportsColor := true
	if os.Getenv("TERM") == "dumb" {
		supportsColor = false
	} else {
		file, isFile := writer.(*os.File)
		if isFile {
			fd := file.Fd()
			supportsColor = isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
		}
	}
	return logger{level, writer, supportsColor}
}

func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

type logger struct {
	level         Level
	writer        io.Writer
	supportsColor bool
}

func (l logger) Level() Level {
	return l.level
}

func (l logger) Infof(format string, a ...interface{}) {
	l.writef(InfoLvl, format+"\n", a...)
}

func (l logger) Verbosef(format string, a ...interface{}) {
	l.writef(VerboseLvl, format+"\n", a...)
}

func (l logger) Debugf(format string, a ...interface{}) {
	l.writef(DebugLvl, format+"\n", a...)
}

func (l logger) writef(level Level, format string, a ...interface{}) {
	if l.level >= level {
		// swallowing errors because:
		// 1) if we can't write to the log, what else are we going to do?
		// 2) a logger interface that returns error becomes really distracting at call sites,
		//    increasing friction and reducing logging
		_, _ = fmt.Fprintf(l.writer, format, a...)
	}
}

func (l logger) Write(level Level, s string) {
	if l.level >= level {
		// swallowing errors because:
		// 1) if we can't write to the log, what else are we going to do?
		// 2) a logger interface that returns error becomes really distracting at call sites,
		//    increasing friction and reducing logging
		_, _ = fmt.Fprintf(l.writer, s)
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

func (l logger) SupportsColor() bool {
	return l.supportsColor
}

func getColor(l Logger, c color.Attribute) *color.Color {
	color := color.New(c)
	if !l.SupportsColor() {
		color.DisableColor()
	}
	return color
}

func Blue(l Logger) *color.Color   { return getColor(l, color.FgBlue) }
func Yellow(l Logger) *color.Color { return getColor(l, color.FgYellow) }
func Green(l Logger) *color.Color  { return getColor(l, color.FgGreen) }
func Red(l Logger) *color.Color    { return getColor(l, color.FgRed) }

// Returns a context containing a logger that forks all of its output
// to both the parent context's logger and to the given `io.Writer`
func CtxWithForkedOutput(ctx context.Context, writer io.Writer) context.Context {
	l := Get(ctx)

	write := func(level Level, b []byte) error {
		l.Write(level, string(b))
		if l.Level() >= level {
			b = append([]byte{}, b...)
			_, err := writer.Write(b)
			if err != nil {
				return err
			}
		}
		return nil
	}

	forkedLogger := funcLogger{
		supportsColor: l.SupportsColor(),
		level:         l.Level(),
		write:         write,
	}

	return WithLogger(ctx, forkedLogger)
}
