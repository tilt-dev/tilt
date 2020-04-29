package os

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/tiltfile/io"
	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestEnviron(t *testing.T) {
	f := NewFixture(t)
	os.Setenv("FAKE_ENV_VARIABLE", "fakeValue")
	defer os.Unsetenv("FAKE_ENV_VARIABLE")

	f.File("Tiltfile", `
print(os.environ['FAKE_ENV_VARIABLE'])
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, "fakeValue\n", f.PrintOutput())
}

func TestGetCwd(t *testing.T) {
	f := NewFixture(t)

	f.File("Tiltfile", `
print(os.getcwd())
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n", f.Path()), f.PrintOutput())
}

func TestGetCwdLoad(t *testing.T) {
	f := NewFixture(t)

	f.File("foo/Tiltfile", `
cwd = os.getcwd()
`)
	f.File("Tiltfile", `
load('./foo/Tiltfile', 'cwd')
print(cwd)
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n", f.JoinPath("foo")), f.PrintOutput())
}

func TestGetCwdLoadFunction(t *testing.T) {
	f := NewFixture(t)

	f.File("foo/Tiltfile", `
def get_cwd_wrapper():
  return os.getcwd()
`)
	f.File("Tiltfile", `
load('./foo/Tiltfile', 'get_cwd_wrapper')
print(get_cwd_wrapper())
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n", f.Path()), f.PrintOutput())
}

func TestAbspath(t *testing.T) {
	f := NewFixture(t)
	f.UseRealFS()

	f.File("foo/Tiltfile", `
path = os.path.abspath('.')
`)
	f.File("Tiltfile", `
load('./foo/Tiltfile', 'path')
print(path)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n", f.JoinPath("foo")), f.PrintOutput())
}

func TestAbspathSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no user-land symlink support on windows")
	}

	f := NewFixture(t)
	f.UseRealFS()

	f.File("foo/Tiltfile", `
path = os.path.abspath('.')
`)
	f.Symlink("foo", "bar")
	f.File("Tiltfile", `
load('./bar/Tiltfile', 'path')
print(path)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n", f.JoinPath("bar")), f.PrintOutput())
}

func TestBasename(t *testing.T) {
	f := NewFixture(t)
	f.UseRealFS()

	f.File("foo/Tiltfile", `
path = os.path.basename(os.path.abspath('.'))
`)
	f.File("Tiltfile", `
load('./foo/Tiltfile', 'path')
print(path)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, "foo\n", f.PrintOutput())
}

func TestDirname(t *testing.T) {
	f := NewFixture(t)
	f.UseRealFS()

	f.File("foo/Tiltfile", `
path = os.path.dirname(os.path.abspath('.'))
`)
	f.File("Tiltfile", `
load('./foo/Tiltfile', 'path')
print(path)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, f.Path()+"\n", f.PrintOutput())
}

func TestExists(t *testing.T) {
	f := NewFixture(t)
	f.UseRealFS()

	f.File("foo/Tiltfile", `
result1 = os.path.exists('foo/Tiltfile')
result2 = os.path.exists('./Tiltfile')
result3 = os.path.exists('../Tiltfile')
result4 = os.path.exists('../foo')
result5 = os.path.exists('../bar')
`)
	f.File("Tiltfile", `
load('./foo/Tiltfile', 'result1', 'result2', 'result3', 'result4', 'result5')
print(result1)
print(result2)
print(result3)
print(result4)
print(result5)
`)

	model, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, "False\nTrue\nTrue\nTrue\nFalse\n", f.PrintOutput())

	readState, err := io.GetState(model)
	require.NoError(t, err)
	assert.Equal(t, f.JoinPath("foo", "foo", "Tiltfile"), readState.Files[2])
}

func TestRealpath(t *testing.T) {
	f := NewFixture(t)

	f.File("Tiltfile", `
print(os.path.realpath('.'))
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n", f.Path()), f.PrintOutput())
}

func TestRealpathSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no user-land symlink support on windows")
	}
	f := NewFixture(t)
	f.UseRealFS()

	f.File("foo/Tiltfile", `
path = os.path.realpath('.')
`)
	f.Symlink("foo", "bar")
	f.File("Tiltfile", `
load('./bar/Tiltfile', 'path')
print(path)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n", f.JoinPath("foo")), f.PrintOutput())
}

func TestName(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
print(os.name)
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("%s\n", osName()), f.PrintOutput())
}

func TestJoin(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
print(os.path.join("foo", "bar", "baz"))
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("%s\n", filepath.Join("foo", "bar", "baz")), f.PrintOutput())
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension(), io.NewExtension())
}
