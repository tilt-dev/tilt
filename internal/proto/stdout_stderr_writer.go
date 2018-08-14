package proto

import (
	"io"
	"os"
)

type StdoutStderrWriter struct {
	stdout io.Writer
	stderr io.Writer
}

func printOutput(output Output) error {
	stdout := output.GetStdout()
	if stdout != nil {
		_, err := os.Stdout.Write(stdout)
		if err != nil {
			return err
		}
	}

	stderr := output.GetStderr()
	if stderr != nil {
		_, err := os.Stderr.Write(stderr)
		if err != nil {
			return err
		}
	}

	return nil
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
