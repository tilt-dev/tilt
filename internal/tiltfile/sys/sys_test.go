package sys

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestArgv(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
print(sys.argv)
`)

	quoted := []string{}
	for _, arg := range os.Args {
		quoted = append(quoted, fmt.Sprintf("%q", arg))
	}
	expected := fmt.Sprintf("[%s]\n", strings.Join(quoted, ", "))

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, expected, f.PrintOutput())
}

func TestExecutable(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
print(sys.executable)
`)

	expected, _ := os.Executable()
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%v\n", expected), f.PrintOutput())
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
