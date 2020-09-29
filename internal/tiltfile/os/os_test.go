package os

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestEnviron(t *testing.T) {
	f := NewFixture(t)
	os.Setenv("FAKE_ENV_VARIABLE", "fakeValue")
	defer os.Unsetenv("FAKE_ENV_VARIABLE")

	f.File("Tiltfile", `
print(os.environ['FAKE_ENV_VARIABLE'])
print(os.environ.get('FAKE_ENV_VARIABLE'))
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, "fakeValue\nfakeValue\n", f.PrintOutput())
}

func TestGetenv(t *testing.T) {
	f := NewFixture(t)
	os.Setenv("FAKE_ENV_VARIABLE", "fakeValue")
	defer os.Unsetenv("FAKE_ENV_VARIABLE")

	f.File("Tiltfile", `
print(os.getenv('FAKE_ENV_VARIABLE'))
print(os.getenv('FAKE_ENV_VARIABLE', 'foo'))
print(os.getenv('FAKE_ENV_VARIABLE_UNSET', 'bar'))
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, "fakeValue\nfakeValue\nbar\n", f.PrintOutput())
}

func TestPutenv(t *testing.T) {
	f := NewFixture(t)
	os.Setenv("FAKE_ENV_VARIABLE", "fakeValue")
	defer os.Unsetenv("FAKE_ENV_VARIABLE")

	f.File("Tiltfile", `
os.putenv('FAKE_ENV_VARIABLE', 'fakeValue2')
print(os.getenv('FAKE_ENV_VARIABLE'))
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, "fakeValue2\n", f.PrintOutput())
	assert.Equal(t, "fakeValue2", os.Getenv("FAKE_ENV_VARIABLE"))
}

func TestPutenvByDict(t *testing.T) {
	f := NewFixture(t)
	os.Setenv("FAKE_ENV_VARIABLE", "fakeValue")
	defer os.Unsetenv("FAKE_ENV_VARIABLE")

	f.File("Tiltfile", `
os.environ['FAKE_ENV_VARIABLE'] = 'fakeValueByDict'
print(os.getenv('FAKE_ENV_VARIABLE'))
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, "fakeValueByDict\n", f.PrintOutput())
	assert.Equal(t, "fakeValueByDict", os.Getenv("FAKE_ENV_VARIABLE"))
}

func TestUnsetenv(t *testing.T) {
	f := NewFixture(t)
	os.Setenv("FAKE_ENV_VARIABLE", "fakeValue")
	defer os.Unsetenv("FAKE_ENV_VARIABLE")

	f.File("Tiltfile", `
os.unsetenv('FAKE_ENV_VARIABLE')
print(os.getenv('FAKE_ENV_VARIABLE', 'unused'))
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, "unused\n", f.PrintOutput())

	_, found := os.LookupEnv("FAKE_ENV_VARIABLE")
	assert.False(t, found)
}

func TestUnsetenvAsDict(t *testing.T) {
	f := NewFixture(t)
	os.Setenv("FAKE_ENV_VARIABLE", "fakeValue")
	defer os.Unsetenv("FAKE_ENV_VARIABLE")

	f.File("Tiltfile", `
os.environ.pop('FAKE_ENV_VARIABLE')
print(os.getenv('FAKE_ENV_VARIABLE', 'unused'))
`)

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, "unused\n", f.PrintOutput())

	_, found := os.LookupEnv("FAKE_ENV_VARIABLE")
	assert.False(t, found)
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

	readState := io.MustState(model)
	assert.Equal(t, f.JoinPath("foo", "foo", "Tiltfile"), readState.Paths[2])
}

func TestPermissionDenied(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test relies on Unix /root permissions")
	}
	f := NewFixture(t)
	f.UseRealFS()

	f.File("Tiltfile", `
print(os.path.exists('/root/x'))
`)

	model, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, "False\n", f.PrintOutput())

	readState := io.MustState(model)
	assert.Equal(t, []string{f.JoinPath("Tiltfile")}, readState.Paths)
}

func TestPathExistsDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test relies on Unix /root permissions")
	}
	f := NewFixture(t)
	f.UseRealFS()

	f.File(f.JoinPath("subdir", "inner", "a.txt"), "hello")
	f.File("Tiltfile", `
print(os.path.exists('subdir'))
`)

	model, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, "True\n", f.PrintOutput())

	// Verify that we're not watching the subdir recursively.
	readState := io.MustState(model)
	assert.Equal(t, []string{f.JoinPath("Tiltfile")}, readState.Paths)
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

	// on MacOS, /tmp is a symlink to /private/tmp. If we don't eval the expected path,
	// we get an error because /tmp != /private/tmp
	expected, err := filepath.EvalSymlinks(f.JoinPath("foo"))
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n", expected), f.PrintOutput())
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
