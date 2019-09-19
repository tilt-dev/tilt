package tiltfile

import (
	"fmt"

	"github.com/windmilleng/tilt/pkg/model"

	"go.starlark.net/starlark"
)

type localResource struct {
	name        string
	cmd         model.Cmd
	deps        []string
	triggerMode triggerMode
	repos       []model.LocalGitRepo
}

func (s *tiltfileState) localResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, cmd string
	var triggerMode triggerMode
	var deps starlark.Value

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"cmd", &cmd,
		"deps?", &deps,
		"trigger_mode?", &triggerMode,
	); err != nil {
		return nil, err
	}

	depsVals := starlarkValueOrSequenceToSlice(deps)
	var depsStrings []string
	for _, v := range depsVals {
		path, err := s.absPathFromStarlarkValue(thread, v)
		if err != nil {
			return nil, fmt.Errorf("deps must be a string or a sequence of strings; found a %T", v)
		}
		depsStrings = append(depsStrings, path)
	}

	repos := reposForPaths(depsStrings)

	res := localResource{
		name:        name,
		cmd:         model.ToShellCmd(cmd),
		deps:        depsStrings,
		triggerMode: triggerMode,
		repos:       repos,
	}
	s.localResources = append(s.localResources, res)

	return starlark.None, nil
}
