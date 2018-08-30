package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/logger"
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

func TestPipeline(t *testing.T) {
	out := &bytes.Buffer{}
	logger := logger.NewLogger(logger.InfoLvl, out)
	o := NewOutputter(logger)
	o.StartPipeline(1)
	o.StartPipelineStep("%s %s", "hello", "world")
	o.Printf("in ur step")
	o.EndPipelineStep()
	o.EndPipeline()

	result := out.String()
	assert.Equal(t, `──┤ Pipeline Starting … ├────────────────────────────────────────
STEP 1/1 — hello world
    ╎ in ur step
    (Done 0.000s)

  │ Step 1 - 0.000s
──┤ ︎Pipeline Done in 0.000s ⚡ ︎├───────────────────────────────────
`, result)
}

func TestMultilinePrintInPipeline(t *testing.T) {
	out := &bytes.Buffer{}
	logger := logger.NewLogger(logger.InfoLvl, out)
	o := NewOutputter(logger)
	o.StartPipeline(1)
	o.StartPipelineStep("%s %s", "hello", "world")
	o.Printf("line 1\nline 2\n")
	o.EndPipelineStep()
	o.EndPipeline()

	result := out.String()
	assert.Equal(t, `──┤ Pipeline Starting … ├────────────────────────────────────────
STEP 1/1 — hello world
    ╎ line 1
    ╎ line 2
    ╎`+
		// The weird syntax here is so that formatters don't strip the trailing whitespace
		" "+
		`
    (Done 0.000s)

  │ Step 1 - 0.000s
──┤ ︎Pipeline Done in 0.000s ⚡ ︎├───────────────────────────────────
`, result)
}

type prefixedWriterTestFixture struct {
	buf    *bytes.Buffer
	writer *prefixedWriter
	t      *testing.T
}

func newPrefixedWriterTestFixture(t *testing.T, prefix string) prefixedWriterTestFixture {
	buf := bytes.NewBuffer(make([]byte, 0))
	return prefixedWriterTestFixture{writer: newPrefixedWriter(prefix, buf), buf: buf, t: t}
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
