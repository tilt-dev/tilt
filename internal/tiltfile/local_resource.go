package tiltfile

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/model"
)

type localResource struct {
	name          string
	updateCmd     model.Cmd
	serveCmd      model.Cmd
	workdir       string
	deps          []string
	triggerMode   triggerMode
	autoInit      bool
	repos         []model.LocalGitRepo
	resourceDeps  []string
	ignores       []string
	allowParallel bool
}

func (s *tiltfileState) localResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var updateCmdVal, updateCmdBatVal, serveCmdVal, serveCmdBatVal starlark.Value
	var triggerMode triggerMode
	var deps starlark.Value
	var resourceDepsVal starlark.Sequence
	var ignoresVal starlark.Value
	var allowParallel bool
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
		"cmd_bat?", &updateCmdBatVal,
		"serve_cmd_bat?", &serveCmdBatVal,
		"allow_parallel?", &allowParallel,
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

	updateCmd, err := value.ValueGroupToCmdHelper(updateCmdVal, updateCmdBatVal)
	if err != nil {
		return nil, err
	}
	serveCmd, err := value.ValueGroupToCmdHelper(serveCmdVal, serveCmdBatVal)
	if err != nil {
		return nil, err
	}

	if updateCmd.Empty() && serveCmd.Empty() {
		return nil, fmt.Errorf("local_resource must have a cmd and/or a serve_cmd, but both were empty")
	}

	res := localResource{
		name:          name,
		updateCmd:     updateCmd,
		serveCmd:      serveCmd,
		workdir:       filepath.Dir(starkit.CurrentExecPath(thread)),
		deps:          depsStrings,
		triggerMode:   triggerMode,
		autoInit:      autoInit,
		repos:         repos,
		resourceDeps:  resourceDeps,
		ignores:       ignores,
		allowParallel: allowParallel,
	}

	//check for duplicate resources by name and throw error if found
	for _, elem := range s.localResources {
		if elem.name == res.name {
			return starlark.None, fmt.Errorf("Local resource %s has been defined multiple times", res.name)
		}
	}
	s.localResources = append(s.localResources, res)

	return starlark.None, nil
}
