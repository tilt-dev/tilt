package logger

import (
	"context"
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
	// log information that is likely to only be of interest to tilt developers
	Debugf(format string, a ...interface{})

	// log information that a tilt user might not want to see on every run, but that they might find
	// useful when debugging their Tiltfile/docker/k8s configs
	Verbosef(format string, a ...interface{})

	// log information that we always want to show
	Infof(format string, a ...interface{})

	// Warnings to show in the alert pane.
	Warnf(format string, a ...interface{})

	// Halting errors to show in the alert pane.
	Errorf(format string, a ...interface{})

	Write(level Level, bytes []byte)

	// gets an io.Writer that filters to the specified level for, e.g., passing to a subprocess
	Writer(level Level) io.Writer

	Level() Level

	SupportsColor() bool

	WithFields(fields Fields) Logger
}

type LogHandler interface {
	Write(level Level, fields Fields, bytes []byte) error
}

type Level struct {
	// UGH, for backwards-compatibility, the serialized value doesn't say anything
	// about relative priority.
	id int32

	severity int32
}

func (l Level) ToProtoID() int32 {
	return l.id
}

// If l is the logger level, determine if we should display
// logs of the given severity.
func (l Level) ShouldDisplay(log Level) bool {
	return l.severity <= log.severity
}

func (l Level) AsSevereAs(log Level) bool {
	return l.severity >= log.severity
}

var (
	NoneLvl    = Level{id: 0, severity: 0}
	DebugLvl   = Level{id: 3, severity: 100}
	VerboseLvl = Level{id: 2, severity: 200}
	InfoLvl    = Level{id: 1, severity: 300}
	WarnLvl    = Level{id: 4, severity: 400}
	ErrorLvl   = Level{id: 5, severity: 500}
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

func NewLogger(minLevel Level, writer io.Writer) Logger {
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
	return NewFuncLogger(supportsColor, minLevel, func(level Level, fields Fields, bytes []byte) error {
		_, err := writer.Write(bytes)
		return err
	})
}

func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
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

func CtxWithLogHandler(ctx context.Context, handler LogHandler) context.Context {
	original := Get(ctx)
	newLogger := NewFuncLogger(original.SupportsColor(), original.Level(), handler.Write)
	return WithLogger(ctx, newLogger)
}

// Returns a context containing a logger that forks all of its output
// to both the parent context's logger and to the given `io.Writer`
func CtxWithForkedOutput(ctx context.Context, writer io.Writer) context.Context {
	l := Get(ctx)

	write := func(level Level, fields Fields, b []byte) error {
		l.Write(level, b)
		if l.Level().ShouldDisplay(level) {
			b = append([]byte{}, b...)
			_, err := writer.Write(b)
			if err != nil {
				return err
			}
		}
		return nil
	}

	forkedLogger := NewFuncLogger(l.SupportsColor(), l.Level(), write)
	return WithLogger(ctx, forkedLogger)
}
