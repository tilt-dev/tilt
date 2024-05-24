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
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const testDeprecationMsg = "test() is deprecated and will be removed in a future release.\n" +
	"Change this call to use `local_resource(..., allow_parallel=True)`"

type localResource struct {
	name      string
	updateCmd model.Cmd
	serveCmd  model.Cmd
	// The working directory of the execution thread where the local resource was created.
	threadDir     string
	deps          []string
	triggerMode   triggerMode
	autoInit      bool
	resourceDeps  []string
	ignores       []string
	allowParallel bool
	links         []model.Link
	labels        map[string]string

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

	var resourceDepsVal starlark.Sequence
	var ignoresVal starlark.Value
	var allowParallel bool
	var links links.LinkList
	var labels value.LabelSet
	var updateStdinMode, serveStdinMode value.StdinMode
	autoInit := true
	if fn.Name() == testN {
		// If we're initializing a test, by default parallelism is on
		allowParallel = true
		logger.Get(s.ctx).Warnf("%s", testDeprecationMsg)
	}

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
		"env?", &updateEnv,
		"serve_env?", &serveEnv,
		"readiness_probe?", &readinessProbe,
		"dir?", &updateCmdDirVal,
		"serve_dir?", &serveCmdDirVal,
		"stdin_mode??", &updateStdinMode,
		"serve_stdin_mode??", &serveStdinMode,
	); err != nil {
		return nil, err
	}

	resourceDeps, err := value.SequenceToStringSlice(resourceDepsVal)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: resource_deps", fn.Name())
	}

	ignores, err := parseValuesToStrings(ignoresVal, "ignore")
	if err != nil {
		return nil, err
	}

	updateCmd, err := value.ValueGroupToCmdHelper(thread,
		updateCmdVal, updateCmdBatVal, updateCmdDirVal, updateEnv,
		updateStdinMode)
	if err != nil {
		return nil, err
	}
	serveCmd, err := value.ValueGroupToCmdHelper(thread,
		serveCmdVal, serveCmdBatVal, serveCmdDirVal, serveEnv,
		serveStdinMode)
	if err != nil {
		return nil, err
	}

	if updateCmd.Empty() && serveCmd.Empty() {
		return nil, fmt.Errorf("local_resource must have a cmd and/or a serve_cmd, but both were empty")
	}

	probeSpec := readinessProbe.Spec()
	if probeSpec != nil && serveCmd.Empty() {
		s.logger.Warnf("Ignoring readiness probe for local resource %q (no serve_cmd was defined)", name)
		probeSpec = nil
	}

	res := &localResource{
		name:           string(name),
		updateCmd:      updateCmd,
		serveCmd:       serveCmd,
		threadDir:      filepath.Dir(starkit.CurrentExecPath(thread)),
		deps:           deps.Value,
		triggerMode:    triggerMode,
		autoInit:       autoInit,
		resourceDeps:   resourceDeps,
		ignores:        ignores,
		allowParallel:  allowParallel,
		links:          links.Links,
		labels:         labels.Values,
		readinessProbe: probeSpec,
	}

	// check for duplicate resources by name and throw error if found
	err = s.checkResourceConflict(res.name)
	if err != nil {
		return nil, err
	}
	s.localResources = append(s.localResources, res)
	s.localByName[res.name] = res

	return starlark.None, nil
}
