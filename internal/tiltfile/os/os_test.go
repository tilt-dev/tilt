package os

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestEnviron(t *testing.T) {
	f := NewFixture(t)
	os.Setenv("FAKE_ENV_VARIABLE", "fakeValue")
	defer os.Unsetenv("FAKE_ENV_VARIABLE")

	f.File("Tiltfile", `
print(os.environ['FAKE_ENV_VARIABLE'])
`)

	err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, "fakeValue\n", f.PrintOutput())
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
