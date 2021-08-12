package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

type FakeBase struct {
	Dir string
}

var _ Base = FakeBase{}

func (b FakeBase) createPath(prefix, relPath string) (string, error) {
	p := filepath.Join(b.Dir, "cache", relPath)
	dir := filepath.Dir(p)
	err := os.MkdirAll(dir, os.ModeDir|0700)
	if err != nil {
		return "", fmt.Errorf("creating dir %s: %v", dir, err)
	}
	return p, nil
}

func (b FakeBase) CacheFile(relPath string) (string, error) {
	return b.createPath("cache", relPath)
}
func (b FakeBase) ConfigFile(relPath string) (string, error) {
	return b.createPath("config", relPath)
}
func (b FakeBase) DataFile(relPath string) (string, error) {
	return b.createPath("data", relPath)
}
func (b FakeBase) StateFile(relPath string) (string, error) {
	return b.createPath("state", relPath)
}
func (b FakeBase) RuntimeFile(relPath string) (string, error) {
	return b.createPath("runtime", relPath)
}
