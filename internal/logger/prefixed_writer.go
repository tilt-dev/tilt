package logger

import (
	"io"
	"strings"
)

// sticks "prefix" at the start of every new line
type prefixedWriter struct {
	prefix                string
	underlying            io.Writer
	indentBeforeNextWrite bool
}

var _ io.Writer = &prefixedWriter{}

func NewPrefixedWriter(prefix string, underlying io.Writer) *prefixedWriter {
	return &prefixedWriter{prefix, underlying, true}
}

func (i *prefixedWriter) Write(buf []byte) (n int, err error) {
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

	_, err = i.underlying.Write([]byte(output))
	if err != nil {
		return 0, err
	}

	return len(buf), nil
}
