package starkit

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestThreadWriter(t *testing.T) {
	f := newThreadWriterFixture(t)

	f.Write("he")
	f.Write("llo\ngoodbye\nforev")
	f.Write("er\n")

	require.Equal(t, "hello\ngoodbye\nforever\n", f.Output())
}

type threadWriterFixture struct {
	w   io.Writer
	out *bytes.Buffer
	t   *testing.T
}

func newThreadWriterFixture(t *testing.T) *threadWriterFixture {
	out := &bytes.Buffer{}
	thread := &starlark.Thread{
		Print: func(thread *starlark.Thread, msg string) {
			out.WriteString(msg + "\n")
		},
	}
	w := NewThreadWriter(thread)
	return &threadWriterFixture{
		w:   w,
		out: out,
		t:   t,
	}
}

func (f *threadWriterFixture) Write(s string) {
	_, err := f.w.Write([]byte(s))
	require.NoError(f.t, err)
}

func (f *threadWriterFixture) Output() string {
	return f.out.String()
}
