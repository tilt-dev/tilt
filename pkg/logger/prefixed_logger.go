package logger

import (
	"strings"
)

// sticks "prefix" at the start of every new line
type prefixedLogger struct {
	Logger

	original              Logger
	prefix                string
	indentBeforeNextWrite bool
}

var _ Logger = &prefixedLogger{}

func NewPrefixedLogger(prefix string, original Logger) *prefixedLogger {
	result := &prefixedLogger{original: original, prefix: prefix, indentBeforeNextWrite: true}

	delegate := NewFuncLogger(original.SupportsColor(), original.Level(), result.handleLog)
	result.Logger = delegate

	return result
}

func (i *prefixedLogger) handleLog(level Level, fields Fields, buf []byte) error {
	output := ""

	if i.indentBeforeNextWrite {
		output += i.prefix
	}

	output += string(buf)

	// temporarily take off a trailing newline so that Replace doesn't add a prefix at the end
	endsInNewline := false
	if len(output) > 0 {
		endsInNewline = output[len(output)-1] == '\n'
	}

	if endsInNewline {
		output = output[:len(output)-1]
	}

	output = strings.Replace(output, "\n", "\n"+i.prefix, -1)

	if endsInNewline {
		output = output + "\n"
		i.indentBeforeNextWrite = true
	} else {
		i.indentBeforeNextWrite = false
	}

	i.original.WithFields(fields).Write(level, []byte(output))
	return nil
}
