package starkit

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

// A fixture for test setup/teardown
type Fixture struct {
	tb               testing.TB
	extensions       []Extension
	path             string
	temp             *tempdir.TempDirFixture
	fs               map[string]string
	out              *bytes.Buffer
	useRealFS        bool // Use a real filesystem
	loadInterceptors []LoadInterceptor
}

func NewFixture(tb testing.TB, extensions ...Extension) *Fixture {
	temp := tempdir.NewTempDirFixture(tb)
	temp.Chdir()

	return &Fixture{
		tb:         tb,
		extensions: extensions,
		path:       temp.Path(),
		temp:       temp,
		fs:         make(map[string]string),
		out:        bytes.NewBuffer(nil),
	}
}

func (f *Fixture) OnStart(e *Environment) error {
	if !f.useRealFS {
		e.SetFakeFileSystem(f.fs)
	}

	e.SetPrint(func(t *starlark.Thread, msg string) {
		_, _ = fmt.Fprintf(f.out, "%s\n", msg)
	})
	e.SetContext(context.Background())
	return nil
}

func (f *Fixture) ExecFile(name string) (Model, error) {
	extensions := append([]Extension{f}, f.extensions...)
	env := newEnvironment(extensions...)
	for _, i := range f.loadInterceptors {
		env.AddLoadInterceptor(i)
	}
	return env.start(filepath.Join(f.path, name))
}

func (f *Fixture) SetLoadInterceptor(i LoadInterceptor) {
	f.loadInterceptors = append(f.loadInterceptors, i)
}

func (f *Fixture) PrintOutput() string {
	return f.out.String()
}

func (f *Fixture) Path() string {
	return f.path
}

func (f *Fixture) JoinPath(elem ...string) string {
	return filepath.Join(append([]string{f.path}, elem...)...)
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
	// '/' is not allowed in filenames, so get that out of there
	path, err := ioutil.TempDir("", strings.Replace(f.tb.Name(), "/", "_", -1))
	require.NoError(f.tb, err)
	f.path = path
	f.useRealFS = true
}

func (f *Fixture) TearDown() {
	f.temp.TearDown()
}
