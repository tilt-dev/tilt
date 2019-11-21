package starkit

import (
	"bytes"
	"io"

	"go.starlark.net/starlark"
)

type threadWriter struct {
	t   *starlark.Thread
	buf *bytes.Buffer
}

func (tw threadWriter) Write(b []byte) (int, error) {
	i := bytes.LastIndexByte(b, '\n')
	if i == -1 {
		return tw.buf.Write(b)
	} else {
		// `:i` to omit the newline, because the thread.Print adds a newline
		out := append(tw.buf.Bytes(), b[:i]...)
		tw.t.Print(tw.t, string(out))
	}
	tw.buf.Reset()
	// Buffer.Write always has nil err
	_, _ = tw.buf.Write(b[i+1:])

	return len(b), nil
}

// a writer that prints to the given starlark Thread's `Print` function
func NewThreadWriter(t *starlark.Thread) io.Writer {
	return threadWriter{t, &bytes.Buffer{}}
}
