package filepath

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	VisitDir(dirname string, newFunc func() runtime.Object, codec runtime.Decoder, visitFunc func(string, runtime.Object) error) error
}

type RealFS struct {
}

var _ FS = RealFS{}

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

func (fs RealFS) VisitDir(dirname string, newFunc func() runtime.Object, codec runtime.Decoder, visitFunc func(string, runtime.Object) error) error {
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
		return visitFunc(path, newObj)
	})
}

// An in-memory structure that pretends to be a filesystem,
// and supports all the storage interfaces that RealFS needs.
type MemoryFS struct {
	mu  sync.Mutex
	dir map[string]interface{}
}

func NewMemoryFS() *MemoryFS {
	return &MemoryFS{
		dir: make(map[string]interface{}),
	}
}

var _ FS = &MemoryFS{}

// Ensures the given directory exists in our in-memory map.
func (fs *MemoryFS) ensureDir(pathToDir string) (map[string]interface{}, error) {
	if pathToDir == "." {
		return fs.dir, nil
	}

	parts := strings.Split(pathToDir, string(filepath.Separator))
	i := 0
	dir := fs.dir
	for i < len(parts) {
		part := parts[i]
		nextDir, ok := dir[part]
		if !ok {
			nextDir = make(map[string]interface{})
			dir[part] = nextDir
		}

		dir, ok = nextDir.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("internal storage error. path conflict. expected map at %s, got: %T", part, nextDir)
		}
		i++
	}
	return dir, nil
}

// Remove the filepath.
func (fs *MemoryFS) Remove(p string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir, err := fs.ensureDir(filepath.Dir(p))
	if err != nil {
		return err
	}
	_, exists := dir[filepath.Base(p)]
	if !exists {
		return os.ErrNotExist
	}

	delete(dir, filepath.Base(p))
	return nil
}

// Check if the filepath exists.
func (fs *MemoryFS) Exists(p string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir, err := fs.ensureDir(filepath.Dir(p))
	if err != nil {
		return false
	}
	_, exists := dir[filepath.Base(p)]
	return exists
}

// Create the directory if it does not exist.
func (fs *MemoryFS) EnsureDir(dirname string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	_, err := fs.ensureDir(dirname)
	return err
}

// Write a copy of the object to our in-memory filesystem.
func (fs *MemoryFS) Write(encoder runtime.Encoder, p string, obj runtime.Object) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir, err := fs.ensureDir(filepath.Dir(p))
	if err != nil {
		return err
	}

	// Encoding the object as bytes ensures that our in-memory filesystem
	// has the same immutability semantics as a real storage system.
	buf := new(bytes.Buffer)
	if err := encoder.Encode(obj, buf); err != nil {
		return err
	}
	dir[filepath.Base(p)] = buf
	return nil
}

// Read a copy of the object from our in-memory filesystem.
func (fs *MemoryFS) Read(decoder runtime.Decoder, p string, newFunc func() runtime.Object) (runtime.Object, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.readInternal(decoder, p, newFunc)
}

func (fs *MemoryFS) readInternal(decoder runtime.Decoder, p string, newFunc func() runtime.Object) (runtime.Object, error) {
	dir, err := fs.ensureDir(filepath.Dir(p))
	if err != nil {
		return nil, err
	}

	contents, ok := dir[filepath.Base(p)]
	if !ok {
		return nil, os.ErrNotExist
	}
	buf, ok := contents.(*bytes.Buffer)
	if !ok {
		return nil, os.ErrNotExist
	}

	newObj := newFunc()
	decodedObj, _, err := decoder.Decode(buf.Bytes(), nil, newObj)
	if err != nil {
		return nil, err
	}
	return decodedObj, nil
}

// Walk the directory, reading all objects in it.
func (fs *MemoryFS) VisitDir(dirname string, newFunc func() runtime.Object, codec runtime.Decoder, visitFunc func(string, runtime.Object) error) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	dir, err := fs.ensureDir(dirname)
	if err != nil {
		return err
	}

	var walk func(ancestorPath string, dir map[string]interface{}) error
	walk = func(ancestorPath string, dir map[string]interface{}) error {
		for key, val := range dir {
			keyPath := filepath.Join(ancestorPath, key)
			innerDir, isDir := val.(map[string]interface{})
			if isDir {
				err = walk(keyPath, innerDir)
				if err != nil {
					return err
				}
				continue
			}

			if !strings.HasSuffix(key, ".json") {
				continue
			}

			newObj, err := fs.readInternal(codec, keyPath, newFunc)
			if err != nil {
				return err
			}
			err = visitFunc(keyPath, newObj)
			if err != nil {
				return err
			}
		}
		return nil
	}

	return walk(dirname, dir)
}
