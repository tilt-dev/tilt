package logger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrefixedWriter(t *testing.T) {
	tf := newPrefixedWriterTestFixture(t, "XXX")
	tf.Write("hello world")
	tf.AssertContentsEqual("XXXhello world")
}

func TestPrefixedWriterStartsWithNewline(t *testing.T) {
	tf := newPrefixedWriterTestFixture(t, "XXX")
	tf.Write("\nhello world")
	tf.AssertContentsEqual("XXX\nXXXhello world")
}

func TestPrefixedWriterEndsWithNewline(t *testing.T) {
	tf := newPrefixedWriterTestFixture(t, "XXX")
	tf.Write("hello world\n")
	tf.Write("foobar")
	tf.AssertContentsEqual("XXXhello world\nXXXfoobar")
}

func TestPrefixedWriterNewlineInMiddle(t *testing.T) {
	tf := newPrefixedWriterTestFixture(t, "XXX")
	tf.Write("hello\nworld")
	tf.Write("foobar")
	tf.AssertContentsEqual("XXXhello\nXXXworldfoobar")
}

type prefixedWriterTestFixture struct {
	buf    *bytes.Buffer
	writer *prefixedWriter
	t      *testing.T
}

func newPrefixedWriterTestFixture(t *testing.T, prefix string) prefixedWriterTestFixture {
	buf := bytes.NewBuffer(make([]byte, 0))
	return prefixedWriterTestFixture{writer: NewPrefixedWriter(prefix, buf), buf: buf, t: t}
}

func (p prefixedWriterTestFixture) Write(s string) {
	n, err := p.writer.Write([]byte(s))
	if err != nil {
		p.t.Fatal(err)
	}

	assert.Equal(p.t, len(s), n)
}

func (p prefixedWriterTestFixture) AssertContentsEqual(expected string) {
	assert.Equal(p.t, expected, p.buf.String())
}
