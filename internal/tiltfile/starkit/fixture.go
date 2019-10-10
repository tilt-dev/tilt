package starkit

import (
	"path/filepath"
	"testing"
)

// A fixture for test setup/teardown
type Fixture struct {
	tb         testing.TB
	extensions []Extension
	path       string
	fs         map[string]string
}

func NewFixture(tb testing.TB, extensions ...Extension) *Fixture {
	return &Fixture{
		tb:         tb,
		extensions: extensions,
		path:       "/fake/path/to/dir",
		fs:         make(map[string]string),
	}
}

func (f *Fixture) OnStart(e *Environment) {
	e.SetFakeFileSystem(f.fs)
}

func (f *Fixture) ExecFile(name string) error {
	extensions := append([]Extension{f}, f.extensions...)
	return ExecFile(filepath.Join(f.path, name), extensions...)
}

func (f *Fixture) Path() string {
	return f.path
}

func (f *Fixture) File(name, contents string) {
	fullPath := filepath.Join(f.path, name)
	f.fs[fullPath] = contents
}
