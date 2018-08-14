package proto

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
)

func TestOutputStreamSuccess(t *testing.T) {
	buf := bytes.Buffer{}
	sendOutput := func(output Output) {
		fmt.Printf("writing '%v'\n", string(output.Stdout))
		buf.Write(output.Stdout)
	}
	outputStream := MakeStdoutStderrWriter(sendOutput)
	outputStream.GetStdoutWriter().Write([]byte("hello"))
	outputStream.GetStdoutWriter().Write([]byte(" world"))
	outputStream.Close()
	assert.Equal(t, "hello world", string(buf.Bytes()))
}

type failingReadCloser struct {
	name       string
	underlying io.ReadCloser
	readFails  bool
	closeFails bool
}

var _ io.ReadCloser = failingReadCloser{}

func (f failingReadCloser) Close() error {
	if f.closeFails {
		return fmt.Errorf("closing %v fails!", f.name)
	} else {
		return f.underlying.Close()
	}
}

func (f failingReadCloser) Read(p []byte) (n int, err error) {
	ret, err := f.underlying.Read(p)

	if f.readFails {
		return 0, fmt.Errorf("reading %v fails!", f.name)
	}

	return ret, err
}

func TestOutputStreamCloseError(t *testing.T) {
	buf := bytes.Buffer{}
	sendOutput := func(output Output) {
		fmt.Printf("writing '%v'\n", string(output.Stdout))
		buf.Write(output.Stdout)
	}
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()
	failingStdoutReader := failingReadCloser{name: "stdout", underlying: stdoutReader, closeFails: true}
	failingStderrReader := failingReadCloser{name: "stderr", underlying: stderrReader, closeFails: true}
	outputStream := makeStdoutStderrWriterFromReaderWriters(sendOutput, failingStdoutReader, stdoutWriter, failingStderrReader, stderrWriter)
	outputStream.GetStdoutWriter().Write([]byte("hello"))
	err := outputStream.Close()
	for _, s := range []string{"closing stderr fails", "closing stdout fails"} {
		assert.True(t, strings.Contains(err.Error(), s), "error '%v' did not contain '%v'", err.Error(), s)
	}
}

func TestOutputStreamReadError(t *testing.T) {
	buf := bytes.Buffer{}
	sendOutput := func(output Output) {
		fmt.Printf("writing '%v'\n", string(output.Stdout))
		buf.Write(output.Stdout)
	}
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()
	failingStdoutReader := failingReadCloser{name: "stdout", underlying: stdoutReader, readFails: true}
	failingStderrReader := failingReadCloser{name: "stderr", underlying: stderrReader, readFails: true}
	outputStream := makeStdoutStderrWriterFromReaderWriters(sendOutput, failingStdoutReader, stdoutWriter, failingStderrReader, stderrWriter)
	outputStream.GetStdoutWriter().Write([]byte("hello"))
	err := outputStream.Close()
	for _, s := range []string{"reading stderr fails", "reading stdout fails"} {
		assert.True(t, strings.Contains(err.Error(), s), "error '%v' did not contain '%v'", err.Error(), s)
	}
}
