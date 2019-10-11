package starkit

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"go.starlark.net/starlark"
)

// A fixture for test setup/teardown
type Fixture struct {
	tb         testing.TB
	extensions []Extension
	path       string
	fs         map[string]string
	out        *bytes.Buffer
}

func NewFixture(tb testing.TB, extensions ...Extension) *Fixture {
	return &Fixture{
		tb:         tb,
		extensions: extensions,
		path:       "/fake/path/to/dir",
		fs:         make(map[string]string),
		out:        bytes.NewBuffer(nil),
	}
}

func (f *Fixture) OnStart(e *Environment) error {
	e.SetFakeFileSystem(f.fs)
	e.SetPrint(func(t *starlark.Thread, msg string) {
		_, _ = fmt.Fprintf(f.out, "%s\n", msg)
	})
	return nil
}

func (f *Fixture) ExecFile(name string) error {
	extensions := append([]Extension{f}, f.extensions...)
	return ExecFile(filepath.Join(f.path, name), extensions...)
}

func (f *Fixture) PrintOutput() string {
	return f.out.String()
}

func (f *Fixture) Path() string {
	return f.path
}

func (f *Fixture) File(name, contents string) {
	fullPath := filepath.Join(f.path, name)
	f.fs[fullPath] = contents
}
