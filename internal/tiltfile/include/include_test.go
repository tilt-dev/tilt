package include

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestLoadError(t *testing.T) {
	f := NewFixture(t)

	f.File("Tiltfile", `
include('./foo/Tiltfile')
`)
	f.File("foo/Tiltfile", `
x = 1
y = x // 0
`)

	_, err := f.ExecFile("Tiltfile")
	if assert.Error(t, err) {
		backtrace := err.(*starlark.EvalError).Backtrace()
		assert.Contains(t, backtrace, fmt.Sprintf("%s:2:8: in <toplevel>", f.JoinPath("Tiltfile")))
		assert.Contains(t, backtrace, fmt.Sprintf("%s:3:7: in <toplevel>", f.JoinPath("foo", "Tiltfile")))
	}
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, &IncludeFn{})
}
