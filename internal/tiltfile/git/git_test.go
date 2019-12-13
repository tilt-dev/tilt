package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

func TestGitRepoPath(t *testing.T) {
	f := NewFixture(t)
	defer f.TearDown()

	f.UseRealFS()
	f.File("Tiltfile", `
print(local_git_repo('.').paths('.git/index'))
`)
	f.File(".git/index", "HEAD")

	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Contains(t, f.PrintOutput(), "/.git/index")
}

func TestGitRepoBadMethodCall(t *testing.T) {
	f := NewFixture(t)
	defer f.TearDown()

	f.UseRealFS()
	f.File("Tiltfile", `
local_git_repo('.').asdf()
`)
	f.File(".git/index", "HEAD")

	_, err := f.ExecFile("Tiltfile")
	if assert.Error(t, err) {
		msg := err.(*starlark.EvalError).Backtrace()
		assert.Contains(t, msg, "Tiltfile:2:20: in <toplevel>")
		assert.Contains(t, msg, "Error: git.Repo has no .asdf field or method")
	}
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
