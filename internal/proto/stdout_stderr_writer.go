package proto

import (
	"bytes"
	"io"
	"k8s.io/apimachinery/pkg/util/errors"
	"os"
)

type StdoutStderrWriter interface {
	GetStdoutWriter() io.Writer
	GetStderrWriter() io.Writer
	Close() error
}

// allows writing of instances of `Output`
type protoStdoutStderrWriter struct {
	stdoutReader, stderrReader io.ReadCloser
	stdoutWriter, stderrWriter io.WriteCloser
	stdoutDone, stderrDone     chan error
}

var _ StdoutStderrWriter = protoStdoutStderrWriter{}

func (s protoStdoutStderrWriter) GetStdoutWriter() io.Writer {
	return s.stdoutWriter
}

func (s protoStdoutStderrWriter) GetStderrWriter() io.Writer {
	return s.stderrWriter
}

func printOutput(output Output) {
	stdout := output.GetStdout()
	if stdout != nil {
		os.Stdout.Write(stdout)
	}

	stderr := output.GetStderr()
	if stderr != nil {
		os.Stderr.Write(stderr)
	}
}

// creates a `StdoutStderrWriter` and starts goroutines that feed data written to the writer to
// the given `sendOutput` function
func MakeStdoutStderrWriter(sendOutput func(Output)) StdoutStderrWriter {
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()
	return makeStdoutStderrWriterFromReaderWriters(sendOutput, stdoutReader, stdoutWriter, stderrReader, stderrWriter)
}

func makeStdoutStderrWriterFromReaderWriters(
	sendOutput func(Output),
	stdoutReader io.ReadCloser,
	stdoutWriter io.WriteCloser,
	stderrReader io.ReadCloser,
	stderrWriter io.WriteCloser) StdoutStderrWriter {
	s := protoStdoutStderrWriter{}
	s.stdoutReader, s.stdoutWriter, s.stderrReader, s.stderrWriter = stdoutReader, stdoutWriter, stderrReader, stderrWriter

	s.stdoutDone, s.stderrDone = make(chan error), make(chan error)

	go func() {
		var buf bytes.Buffer
		for {
			n, err := buf.ReadFrom(s.stdoutReader)
			if err != nil {
				s.stdoutDone <- err
				return
			}
			if n == 0 {
				s.stdoutDone <- nil
				return
			}
			sendOutput(Output{Stdout: buf.Bytes()})
		}
	}()

	go func() {
		var buf bytes.Buffer
		for {
			n, err := buf.ReadFrom(s.stderrReader)
			if err != nil {
				s.stderrDone <- err
				return
			}
			if n == 0 {
				s.stderrDone <- nil
				return
			}
			sendOutput(Output{Stderr: buf.Bytes()})
		}
	}()

	return s
}

func (s protoStdoutStderrWriter) Close() error {
	ret := make([]error, 0)
	for _, closer := range []io.Closer{s.stdoutWriter, s.stderrWriter} {
		err := closer.Close()
		if err != nil {
			ret = append(ret, err)
		}
	}

	stdoutErr, stderrErr := <-s.stdoutDone, <-s.stderrDone

	if stdoutErr != nil {
		ret = append(ret, stdoutErr)
	}

	if stderrErr != nil {
		ret = append(ret, stderrErr)
	}

	for _, closer := range []io.Closer{s.stdoutReader, s.stderrReader} {
		err := closer.Close()
		if err != nil {
			ret = append(ret, err)
		}
	}

	switch len(ret) {
	case 0:
		return nil
	case 1:
		return ret[0]
	default:
		return errors.NewAggregate(ret)
	}
}
