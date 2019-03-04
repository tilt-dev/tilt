package output

import (
	"io"
	"os"
)

var OriginalStderr *os.File

func init() {
	OriginalStderr = os.Stderr
}

func CaptureAllOutput(to io.Writer) error {
	piper, pipew, err := os.Pipe()
	if err != nil {
		return err
	}

	os.Stdout = piper
	os.Stderr = piper

	go func() {
		// NOTE(dmiller): If this errors there's nothing we can do
		_, _ = io.Copy(to, pipew)
	}()
	return nil
}
