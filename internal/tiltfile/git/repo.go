package git

import (
	"errors"
	"fmt"
	"path/filepath"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
)

type Repo struct {
	basePath string
}

var _ starlark.Value = &Repo{}

func (gr *Repo) String() string {
	return fmt.Sprintf("[git.Repo] '%v'", gr.basePath)
}

func (gr *Repo) Type() string {
	return "git.Repo"
}

func (gr *Repo) Freeze() {}

func (gr *Repo) Truth() starlark.Bool {
	return gr.basePath != ""
}

func (*Repo) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: git.Repo")
}

func (gr *Repo) Attr(name string) (starlark.Value, error) {
	switch name {
	case "paths":
		return starlark.NewBuiltin(name, gr.path), nil
	default:
		return nil, nil
	}

}

func (gr *Repo) AttrNames() []string {
	return []string{"paths"}
}

func (gr *Repo) path(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs, "paths", &path)
	if err != nil {
		return nil, err
	}

	return starlark.String(gr.MakeLocalPath(path)), nil
}

func (gr *Repo) MakeLocalPath(path string) string {
	return filepath.Join(gr.basePath, path)
}

var _ value.PathMaker = &Repo{}
