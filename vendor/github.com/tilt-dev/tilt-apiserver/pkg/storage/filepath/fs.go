package filepath

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
)

var VersionError = errors.New("incorrect object version")

// A filesystem interface so we can sub out filesystem-based storage
// with memory-based storage.
type FS interface {
	Remove(filepath string) error
	Exists(filepath string) bool
	EnsureDir(dirname string) error
	Write(encoder runtime.Encoder, filepath string, obj runtime.Object, storageVersion uint64) error
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

func (fs RealFS) Write(encoder runtime.Encoder, filepath string, obj runtime.Object, storageVersion uint64) error {
	// TODO(milas): use storageVersion to ensure we don't perform stale writes
	// 	(currently, this isn't a critical priority as our use cases that rely
	// 	on RealFS do not have simultaneous writers)
	if err := setResourceVersion(obj, storageVersion+1); err != nil {
		return err
	}

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
	rev uint64
}

func NewMemoryFS() *MemoryFS {
	return &MemoryFS{
		dir: make(map[string]interface{}),
	}
}

var _ FS = &MemoryFS{}

type versionedData struct {
	version uint64
	data    []byte
}

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
func (fs *MemoryFS) Write(encoder runtime.Encoder, p string, obj runtime.Object, storageVersion uint64) error {
	// use a copy of the object w/o a resource version for
	// serialization, so that objects that are identical besides resource
	// version serialize the same, allowing us to skip unnecessary writes
	// (beneficial for preventing unnecessary optimistic concurrency failures
	// where an update fails because of a changed version despite the object
	// actually being identical)
	versionlessObj := obj.DeepCopyObject()
	if err := clearResourceVersion(versionlessObj); err != nil {
		return err
	}

	// Encoding the object as bytes ensures that our in-memory filesystem
	// has the same immutability semantics as a real storage system.
	buf := new(bytes.Buffer)
	if err := encoder.Encode(versionlessObj, buf); err != nil {
		return err
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	if rawObj, err := fs.readBuffer(p); err != nil {
		if os.IsNotExist(err) {
			// storageVersion == 0 -> this is a create, so it's expected to not exist (continue)
			// storageVersion != 0 -> object has been deleted, propagate err to avoid a zombie update
			if storageVersion != 0 {
				return err
			}
		} else {
			return err
		}
	} else if rawObj.version != storageVersion {
		// this write is outdated
		return VersionError
	} else if bytes.Equal(rawObj.data, buf.Bytes()) {
		// object serialized identically, skip write & version increment
		return nil
	}

	dir, err := fs.ensureDir(filepath.Dir(p))
	if err != nil {
		return err
	}

	// increment the resource version - it's applied to the object pointer for
	// the caller in addition to being used to ensure the write is valid
	newVersion := fs.incrementRev()
	if err := setResourceVersion(obj, newVersion); err != nil {
		return err
	}

	dir[filepath.Base(p)] = versionedData{
		version: newVersion,
		data:    buf.Bytes(),
	}

	return nil
}

// Read a copy of the object from our in-memory filesystem.
func (fs *MemoryFS) Read(decoder runtime.Decoder, p string, newFunc func() runtime.Object) (runtime.Object, error) {
	fs.mu.Lock()
	buf, err := fs.readBuffer(p)
	fs.mu.Unlock()
	if err != nil {
		return nil, err
	}

	obj, err := fs.decodeBuffer(decoder, buf, newFunc)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (fs *MemoryFS) readBuffer(p string) (versionedData, error) {
	dir, err := fs.ensureDir(filepath.Dir(p))
	if err != nil {
		return versionedData{}, err
	}

	contents, ok := dir[filepath.Base(p)]
	if !ok {
		return versionedData{}, os.ErrNotExist
	}
	data, ok := contents.(versionedData)
	if !ok {
		return versionedData{}, os.ErrNotExist
	}
	return data, nil
}

func (fs *MemoryFS) decodeBuffer(decoder runtime.Decoder, rawObj versionedData, newFunc func() runtime.Object) (runtime.Object, error) {
	newObj := newFunc()
	decodedObj, _, err := decoder.Decode(rawObj.data, nil, newObj)
	if err != nil {
		return nil, err
	}
	if err := setResourceVersion(decodedObj, rawObj.version); err != nil {
		return nil, err
	}
	return decodedObj, nil
}

// Walk the directory, reading all objects in it.
func (fs *MemoryFS) VisitDir(dirname string, newFunc func() runtime.Object, codec runtime.Decoder, visitFunc func(string, runtime.Object) error) error {
	fs.mu.Lock()
	keyPaths, buffers, err := fs.readDir(dirname)
	fs.mu.Unlock()
	if err != nil {
		return err
	}

	// Do decoding and visitation outside the lock.
	for i, keyPath := range keyPaths {
		buf := buffers[i]
		obj, err := fs.decodeBuffer(codec, buf, newFunc)
		if err != nil {
			return err
		}
		err = visitFunc(keyPath, obj)
		if err != nil {
			return err
		}
	}
	return nil
}

// Internal helper for reading the directory. Must hold the mutex.
func (fs *MemoryFS) readDir(dirname string) ([]string, []versionedData, error) {
	dir, err := fs.ensureDir(dirname)
	if err != nil {
		return nil, nil, err
	}

	keyPaths := []string{}
	buffers := []versionedData{}

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

			rawObj, err := fs.readBuffer(keyPath)
			if err != nil {
				return err
			}
			keyPaths = append(keyPaths, keyPath)
			buffers = append(buffers, rawObj)
		}
		return nil
	}

	err = walk(dirname, dir)
	return keyPaths, buffers, err
}

// incrementRev increases the revision counter and returns the new value.
//
// mu must be held.
func (fs *MemoryFS) incrementRev() uint64 {
	fs.rev++
	return fs.rev
}
