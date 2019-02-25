package output

import (
	"io"
	"io/ioutil"
	"log"
	"os"
)

func CaptureAll() io.Writer {
	tmpfile, err := ioutil.TempFile("", "tilt.output.*")
	if err != nil {
		log.Fatal(err)
	}

	os.Stdout = tmpfile
	os.Stderr = tmpfile

	return tmpfile
}
