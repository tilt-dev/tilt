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
	updateCmd    model.Cmd
	serveCmd     model.Cmd
	workdir      string
	deps         []string
	triggerMode  triggerMode
	autoInit     bool
	repos        []model.LocalGitRepo
	resourceDeps []string
	ignores      []string
}

func (s *tiltfileState) localResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var updateCmdVal, serveCmdVal starlark.Value
	var triggerMode triggerMode
	var deps starlark.Value
	var resourceDepsVal starlark.Sequence
	var ignoresVal starlark.Value
	autoInit := true

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"cmd?", &updateCmdVal,
		"deps?", &deps,
		"trigger_mode?", &triggerMode,
		"resource_deps?", &resourceDepsVal,
		"ignore?", &ignoresVal,
		"auto_init?", &autoInit,
		"serve_cmd?", &serveCmdVal,
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

	updateCmd, err := value.ValueToCmd(updateCmdVal)
	if err != nil {
		return nil, err
	}
	serveCmd, err := value.ValueToCmd(serveCmdVal)
	if err != nil {
		return nil, err
	}

	if updateCmd.Empty() && serveCmd.Empty() {
		return nil, fmt.Errorf("local_resource must have a cmd and/or a serve_cmd, but both were empty")
	}

	res := localResource{
		name:         name,
		updateCmd:    updateCmd,
		serveCmd:     serveCmd,
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
