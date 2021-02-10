package filepath

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
)

// A filesystem interface so we can sub out filesystem-based storage
// with memory-based storage.
type FS interface {
	Remove(filepath string) error
	Exists(filepath string) bool
	EnsureDir(dirname string) error
	Write(encoder runtime.Encoder, filepath string, obj runtime.Object) error
	Read(decoder runtime.Decoder, path string, newFunc func() runtime.Object) (runtime.Object, error)
	VisitDir(dirname string, newFunc func() runtime.Object, codec runtime.Decoder, visitFunc func(string, runtime.Object)) error
}

type RealFS struct {
}

func (fs RealFS) Remove(filepath string) error {
	return os.Remove(filepath)
}

func (fs RealFS) Exists(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil
}

func (fs RealFS) EnsureDir(dirname string) error {
	if !fs.Exists(dirname) {
		return os.MkdirAll(dirname, 0700)
	}
	return nil
}

func (fs RealFS) Write(encoder runtime.Encoder, filepath string, obj runtime.Object) error {
	buf := new(bytes.Buffer)
	if err := encoder.Encode(obj, buf); err != nil {
		return err
	}
	return ioutil.WriteFile(filepath, buf.Bytes(), 0600)
}

func (fs RealFS) Read(decoder runtime.Decoder, path string, newFunc func() runtime.Object) (runtime.Object, error) {
	content, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	newObj := newFunc()
	decodedObj, _, err := decoder.Decode(content, nil, newObj)
	if err != nil {
		return nil, err
	}
	return decodedObj, nil
}

func (fs RealFS) VisitDir(dirname string, newFunc func() runtime.Object, codec runtime.Decoder, visitFunc func(string, runtime.Object)) error {
	return filepath.Walk(dirname, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}
		newObj, err := fs.Read(codec, path, newFunc)
		if err != nil {
			return err
		}
		visitFunc(path, newObj)
		return nil
	})
}
