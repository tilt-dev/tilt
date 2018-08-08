package tiltfile

import (
	"testing"
	"io/ioutil"
	"log"
	"os"
	"github.com/stretchr/testify/assert"
)

func tempFile(content string) string {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.WriteString(content)
	if err != nil {
		log.Fatal(err)
	}

	return f.Name()
}

func TestLoadFunctions(t *testing.T) {
	file := tempFile(
`def blorgly():
  return "blorgly"

def blorgly_backend():
  return "blorgly_backend"

def blorgly_frontend():
  return "blorgly_frontend"
`)
	defer os.Remove(file)
	tiltConfig := Load(file)
	for _, s := range([]string{"blorgly", "blorgly_backend", "blorgly_frontend"}) {
		assert.Contains(t, tiltConfig.globals, s)
	}
}
