package os

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestRealpath(t *testing.T) {
	f := NewFixture(t)

	f.File("Tiltfile", `
print(os.path.realpath('.'))
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n", f.Path()), f.PrintOutput())
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

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
