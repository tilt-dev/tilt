package logger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrefixedLogger(t *testing.T) {
	tf := newPrefixedLoggerTestFixture(t, "XXX")
	tf.Write("hello world")
	tf.AssertContentsEqual("XXXhello world")
}

func TestPrefixedLoggerStartsWithNewline(t *testing.T) {
	tf := newPrefixedLoggerTestFixture(t, "XXX")
	tf.Write("\nhello world")
	tf.AssertContentsEqual("XXX\nXXXhello world")
}

func TestPrefixedLoggerEndsWithNewline(t *testing.T) {
	tf := newPrefixedLoggerTestFixture(t, "XXX")
	tf.Write("hello world\n")
	tf.Write("foobar")
	tf.AssertContentsEqual("XXXhello world\nXXXfoobar")
}

func TestPrefixedLoggerNewlineInMiddle(t *testing.T) {
	tf := newPrefixedLoggerTestFixture(t, "XXX")
	tf.Write("hello\nworld")
	tf.Write("foobar")
	tf.AssertContentsEqual("XXXhello\nXXXworldfoobar")
}

type prefixedLoggerTestFixture struct {
	buf    *bytes.Buffer
	logger *prefixedLogger
	t      *testing.T
}

func newPrefixedLoggerTestFixture(t *testing.T, prefix string) prefixedLoggerTestFixture {
	buf := bytes.NewBuffer(make([]byte, 0))
	original := NewLogger(InfoLvl, buf)
	return prefixedLoggerTestFixture{logger: NewPrefixedLogger(prefix, original), buf: buf, t: t}
}

func (p prefixedLoggerTestFixture) Write(s string) {
	p.logger.Write(InfoLvl, []byte(s))
}

func (p prefixedLoggerTestFixture) AssertContentsEqual(expected string) {
	assert.Equal(p.t, expected, p.buf.String())
}
