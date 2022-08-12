package starkit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// A fixture for test setup/teardown
type Fixture struct {
	tb               testing.TB
	plugins          []Plugin
	path             string
	temp             *tempdir.TempDirFixture
	fs               map[string]string
	out              *bytes.Buffer
	useRealFS        bool // Use a real filesystem
	loadInterceptors []LoadInterceptor
	ctx              context.Context
	tf               *v1alpha1.Tiltfile
}

func NewFixture(tb testing.TB, plugins ...Plugin) *Fixture {
	out := bytes.NewBuffer(nil)
	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(out))
	temp := tempdir.NewTempDirFixture(tb)
	temp.Chdir()

	ret := &Fixture{
		tb:      tb,
		plugins: plugins,
		path:    temp.Path(),
		temp:    temp,
		fs:      make(map[string]string),
		out:     out,
		ctx:     ctx,
		tf: &v1alpha1.Tiltfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: "(Tiltfile)",
			},
		},
	}

	return ret
}

func (f *Fixture) SetContext(ctx context.Context) {
	f.ctx = ctx
}

func (f *Fixture) SetOutput(out *bytes.Buffer) {
	f.out = out
}

func (f *Fixture) OnStart(e *Environment) error {
	if !f.useRealFS {
		e.SetFakeFileSystem(f.fs)
	}

	e.SetPrint(func(t *starlark.Thread, msg string) {
		_, _ = fmt.Fprintf(f.out, "%s\n", msg)
	})
	e.SetContext(f.ctx)
	return nil
}

func (f *Fixture) ExecFile(name string) (Model, error) {
	plugins := append([]Plugin{f}, f.plugins...)
	env := newEnvironment(plugins...)
	for _, i := range f.loadInterceptors {
		env.AddLoadInterceptor(i)
	}
	f.tf.Spec.Path = filepath.Join(f.path, name)
	return env.start(f.tf)
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

func (f *Fixture) Tiltfile() *v1alpha1.Tiltfile {
	return f.tf
}

func (f *Fixture) JoinPath(elem ...string) string {
	return filepath.Join(append([]string{f.path}, elem...)...)
}

func (f *Fixture) File(name, contents string) {
	fullPath := name
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(f.path, name)
	}

	if f.useRealFS {
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, os.FileMode(0755))
		assert.NoError(f.tb, err)

		err = os.WriteFile(fullPath, []byte(contents), os.FileMode(0644))
		assert.NoError(f.tb, err)
		return
	}
	f.fs[fullPath] = contents
}

func (f *Fixture) Symlink(old, new string) {
	if !f.useRealFS {
		panic("Can only use symlinks with a real FS")
	}
	err := os.Symlink(f.JoinPath(old), f.JoinPath(new))
	assert.NoError(f.tb, err)
}

func (f *Fixture) UseRealFS() {
	path, err := os.MkdirTemp("", tempdir.SanitizeFileName(f.tb.Name()))
	require.NoError(f.tb, err)
	f.path = path
	f.useRealFS = true
	f.tf.Spec.Path = path
}
