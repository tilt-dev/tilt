package tiltfile

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/links"
	"github.com/tilt-dev/tilt/internal/tiltfile/probe"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"

	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type localResource struct {
	name      string
	updateCmd model.Cmd
	serveCmd  model.Cmd
	// The working directory of the execution thread where the local resource was created.
	threadDir     string
	deps          []string
	triggerMode   triggerMode
	autoInit      bool
	repos         []model.LocalGitRepo
	resourceDeps  []string
	ignores       []string
	allowParallel bool
	links         []model.Link
	labels        []string

	// for use in testing mvp
	tags   []string
	isTest bool

	readinessProbe *v1alpha1.Probe
}

func (s *tiltfileState) localResource(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name value.Name
	var updateCmdVal, updateCmdBatVal, serveCmdVal, serveCmdBatVal starlark.Value
	var updateEnv, serveEnv value.StringStringMap
	var triggerMode triggerMode
	var readinessProbe probe.Probe
	var updateCmdDirVal, serveCmdDirVal starlark.Value

	deps := value.NewLocalPathListUnpacker(thread)

	var resourceDepsVal, tagsVal starlark.Sequence
	var ignoresVal starlark.Value
	var allowParallel bool
	var links links.LinkList
	var labels value.LabelOrLabelList
	autoInit := true

	var isTest bool
	if fn.Name() == testN {
		// If we're initializing a test, by default parallelism is on
		allowParallel = true

		isTest = true

		// TODO: implement timeout
		//   (Maybe all local resources should accept a timeout, not just tests?)
	}

	// TODO: using this func for both `local_resource()` and `test()`, but in future
	//   we should probably unpack args separately
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
		"links?", &links,
		"labels?", &labels,
		"tags?", &tagsVal,
		"env?", &updateEnv,
		"serve_env?", &serveEnv,
		"readiness_probe?", &readinessProbe,
		"dir?", &updateCmdDirVal,
		"serve_dir?", &serveCmdDirVal,
	); err != nil {
		return nil, err
	}

	repos := reposForPaths(deps.Value)

	resourceDeps, err := value.SequenceToStringSlice(resourceDepsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: resource_deps", fn.Name())
	}
	tags, err := value.SequenceToStringSlice(tagsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: resource_deps", fn.Name())
	}

	ignores, err := parseValuesToStrings(ignoresVal, "ignore")
	if err != nil {
		return nil, err
	}

	updateCmd, err := value.ValueGroupToCmdHelper(thread, updateCmdVal, updateCmdBatVal, updateCmdDirVal, updateEnv)
	if err != nil {
		return nil, err
	}
	serveCmd, err := value.ValueGroupToCmdHelper(thread, serveCmdVal, serveCmdBatVal, serveCmdDirVal, serveEnv)
	if err != nil {
		return nil, err
	}

	if updateCmd.Empty() && serveCmd.Empty() {
		return nil, fmt.Errorf("local_resource must have a cmd and/or a serve_cmd, but both were empty")
	}

	res := localResource{
		name:           string(name),
		updateCmd:      updateCmd,
		serveCmd:       serveCmd,
		threadDir:      filepath.Dir(starkit.CurrentExecPath(thread)),
		deps:           deps.Value,
		triggerMode:    triggerMode,
		autoInit:       autoInit,
		repos:          repos,
		resourceDeps:   resourceDeps,
		ignores:        ignores,
		allowParallel:  allowParallel,
		links:          links.Links,
		labels          labels.Values,
		tags:           tags,
		isTest:         isTest,
		readinessProbe: readinessProbe.Spec(),
	}

	// check for duplicate resources by name and throw error if found
	for _, elem := range s.localResources {
		if elem.name == res.name {
			return starlark.None, fmt.Errorf("Local resource %s has been defined multiple times", res.name)
		}
	}
	s.localResources = append(s.localResources, res)

	return starlark.None, nil
}
