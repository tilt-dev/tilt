package output

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/logger"
)

// NOTE(dmiller): set at runtime with:
// go test -ldflags="-X github.com/windmilleng/tilt/internal/build.WriteGoldenMaster=1" github.com/windmilleng/tilt/internal/build -run ^TestBuildkitPrinter
var WriteGoldenMaster = "0"

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
	var err error
	out := &bytes.Buffer{}
	l := logger.NewLogger(logger.InfoLvl, out)
	o := NewOutputter(l)
	o.StartPipeline(1)
	o.StartPipelineStep("%s %s", "hello", "world")
	o.Printf("in ur step")
	o.EndPipelineStep()
	o.EndPipeline(err)

	assertSnapshot(t, out.String())
}

func TestErroredPipeline(t *testing.T) {
	err := fmt.Errorf("oh noes")
	out := &bytes.Buffer{}
	l := logger.NewLogger(logger.InfoLvl, out)
	o := NewOutputter(l)
	o.StartPipeline(1)
	o.StartPipelineStep("%s %s", "hello", "world")
	o.Printf("in ur step")
	o.EndPipelineStep()
	o.EndPipeline(err)

	assertSnapshot(t, out.String())
}

func TestMultilinePrintInPipeline(t *testing.T) {
	var err error
	out := &bytes.Buffer{}
	l := logger.NewLogger(logger.InfoLvl, out)
	o := NewOutputter(l)
	o.StartPipeline(1)
	o.StartPipelineStep("%s %s", "hello", "world")
	o.Printf("line 1\nline 2\n")
	o.EndPipelineStep()
	o.EndPipeline(err)

	assertSnapshot(t, out.String())
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

func assertSnapshot(t *testing.T, output string) {
	d1 := []byte(output)
	gmPath := fmt.Sprintf("testdata/%s_master", t.Name())
	if WriteGoldenMaster == "1" {
		err := ioutil.WriteFile(gmPath, d1, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	expected, err := ioutil.ReadFile(gmPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(expected) != output {
		t.Errorf("Expected: %s != Output: %s", expected, output)
	}
}
