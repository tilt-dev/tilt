package output

import (
	"io/ioutil"
	"log"
	"os"
)

type Stdout *os.File

func CaptureAll() Stdout {
	tmpfile, err := ioutil.TempFile("", "tilt.output.*")
	if err != nil {
		log.Fatal(err)
	}

	os.Stdout = tmpfile
	os.Stderr = tmpfile

	return tmpfile
}
