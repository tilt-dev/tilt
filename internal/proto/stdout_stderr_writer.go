package proto

import (
	"io"
	"k8s.io/apimachinery/pkg/util/errors"
	"os"
)

type StdoutStderrWriter struct {
	stdout io.Writer
	stderr io.Writer
}

func printOutput(output Output) error {
	errs := make([]error, 0)

	stdout := output.GetStdout()
	if stdout != nil {
		_, err := os.Stdout.Write(stdout)
		if err != nil {
			errs = append(errs, err)
		}
	}

	stderr := output.GetStderr()
	if stderr != nil {
		_, err := os.Stderr.Write(stderr)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}

type outputWriter struct {
	sendBytes func([]byte) error
}

var _ io.Writer = outputWriter{}

func (s outputWriter) Write(b []byte) (n int, err error) {
	return len(b), s.sendBytes(b)
}

func MakeStdoutStderrWriter(sendOutput func(Output) error) StdoutStderrWriter {
	stdout := outputWriter{sendBytes: func(b []byte) error { return sendOutput(Output{Stdout: b}) }}
	stderr := outputWriter{sendBytes: func(b []byte) error { return sendOutput(Output{Stderr: b}) }}
	return StdoutStderrWriter{stdout, stderr}
}
