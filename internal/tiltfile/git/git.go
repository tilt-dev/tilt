package git

import (
	"fmt"
	"os"
	"path/filepath"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type Extension struct{}

func NewExtension() Extension {
	return Extension{}
}

func (Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("local_git_repo", localGitRepo)
}

func NewGitRepo(t *starlark.Thread, path string) (*Repo, error) {
	absPath := starkit.AbsPath(t, path)
	_, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("Reading paths %s: %v", absPath, err)
	}

	if _, err := os.Stat(filepath.Join(absPath, ".git")); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s isn't a valid git repo: it doesn't have a .git/ directory", absPath)
	}

	return &Repo{basePath: absPath}, nil
}

func localGitRepo(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	return NewGitRepo(thread, path)
}
