package starkit

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.starlark.net/starlark"
)

// A fixture for test setup/teardown
type Fixture struct {
	tb         testing.TB
	extensions []Extension
	path       string
	fs         map[string]string
	out        *bytes.Buffer
	useRealFS  bool // Use a real filesystem
	args       []string
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

func (f *Fixture) SetArgs(args ...string) {
	f.args = args
}

func (f *Fixture) OnStart(e *Environment) error {
	if !f.useRealFS {
		e.SetFakeFileSystem(f.fs)
	}

	e.SetPrint(func(t *starlark.Thread, msg string) {
		_, _ = fmt.Fprintf(f.out, "%s\n", msg)
	})
	return nil
}

func (f *Fixture) ExecFile(name string) (Model, error) {
	extensions := append([]Extension{f}, f.extensions...)
	return ExecFile(filepath.Join(f.path, name), f.args, extensions...)
}

func (f *Fixture) PrintOutput() string {
	return f.out.String()
}

func (f *Fixture) Path() string {
	return f.path
}

func (f *Fixture) File(name, contents string) {
	fullPath := filepath.Join(f.path, name)
	if f.useRealFS {
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, os.FileMode(0755))
		assert.NoError(f.tb, err)

		err = ioutil.WriteFile(fullPath, []byte(contents), os.FileMode(0644))
		assert.NoError(f.tb, err)
		return
	}
	f.fs[fullPath] = contents
}

func (f *Fixture) UseRealFS() {
	path, err := ioutil.TempDir("", f.tb.Name())
	assert.NoError(f.tb, err)
	f.path = path
	f.useRealFS = true
}
