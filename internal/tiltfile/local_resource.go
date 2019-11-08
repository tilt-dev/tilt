package tiltfile

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/internal/tiltfile/value"
	"github.com/windmilleng/tilt/pkg/model"
)

type localResource struct {
	name         string
	cmd          model.Cmd
	workdir      string
	deps         []string
	triggerMode  triggerMode
	autoInit     bool
	repos        []model.LocalGitRepo
	resourceDeps []string
	ignores      []string
}

func (s *tiltfileState) localResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, cmd string
	var triggerMode triggerMode
	var deps starlark.Value
	var resourceDepsVal starlark.Sequence
	var ignoresVal starlark.Value
	autoInit := true

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"cmd", &cmd,
		"deps?", &deps,
		"trigger_mode?", &triggerMode,
		"resource_deps?", &resourceDepsVal,
		"ignore?", &ignoresVal,
		"auto_init?", &autoInit,
	); err != nil {
		return nil, err
	}

	depsVals := starlarkValueOrSequenceToSlice(deps)
	var depsStrings []string
	for _, v := range depsVals {
		path, err := value.ValueToAbsPath(thread, v)
		if err != nil {
			return nil, fmt.Errorf("deps must be a string or a sequence of strings; found a %T", v)
		}
		depsStrings = append(depsStrings, path)
	}

	repos := reposForPaths(depsStrings)

	resourceDeps, err := value.SequenceToStringSlice(resourceDepsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: resource_deps", fn.Name())
	}

	ignores, err := parseValuesToStrings(ignoresVal, "ignore")
	if err != nil {
		return nil, err
	}

	res := localResource{
		name:         name,
		cmd:          model.ToShellCmd(cmd),
		workdir:      filepath.Dir(starkit.CurrentExecPath(thread)),
		deps:         depsStrings,
		triggerMode:  triggerMode,
		autoInit:     autoInit,
		repos:        repos,
		resourceDeps: resourceDeps,
		ignores:      ignores,
	}
	s.localResources = append(s.localResources, res)

	return starlark.None, nil
}
