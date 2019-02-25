package output

import (
	"io/ioutil"
	"log"
	"os"
)

var outputFile *os.File

func CaptureAll() {
	if outputFile != nil {
		return
	}
	tmpfile, err := ioutil.TempFile("", "tilt.stdout.*")
	if err != nil {
		log.Fatal(err)
	}
	outputFile = tmpfile

	os.Stdout = tmpfile
	os.Stderr = tmpfile
}

func File() *os.File {
	return outputFile
}
