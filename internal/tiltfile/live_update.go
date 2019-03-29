package tiltfile

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"go.starlark.net/syntax"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/model"
)

type liveUpdateDef struct {
	imageName           string
	steps               []model.LiveUpdateStep
	fullRebuildTriggers []string

	// whether this has been matched to a deployed image
	matched bool
}

type liveUpdateStep interface {
	starlark.Value
	liveUpdateStep()
	declarationPosition() syntax.Position
}

var _ starlark.Value = liveUpdateWorkDirStep{}
var _ liveUpdateStep = liveUpdateWorkDirStep{}

type liveUpdateWorkDirStep struct {
	value    string
	position syntax.Position
}

func (l liveUpdateWorkDirStep) String() string                       { return strconv.Quote(string(l.value)) }
func (l liveUpdateWorkDirStep) Type() string                         { return "live_update_workdir_step" }
func (l liveUpdateWorkDirStep) Freeze()                              {}
func (l liveUpdateWorkDirStep) Truth() starlark.Bool                 { return len(l.value) > 0 }
func (l liveUpdateWorkDirStep) Hash() (uint32, error)                { return starlark.String(l.value).Hash() }
func (l liveUpdateWorkDirStep) liveUpdateStep()                      {}
func (l liveUpdateWorkDirStep) declarationPosition() syntax.Position { return l.position }

type liveUpdateSyncStep struct {
	// remotePath is potentially relative in this struct, because we don't know if there's a workDir
	localPath, remotePath string
	position              syntax.Position
}

var _ starlark.Value = liveUpdateSyncStep{}
var _ liveUpdateStep = liveUpdateSyncStep{}

func (l liveUpdateSyncStep) String() string {
	return fmt.Sprintf("'%s'->'%s'", l.localPath, l.remotePath)
}
func (l liveUpdateSyncStep) Type() string { return "live_update_sync_step" }
func (l liveUpdateSyncStep) Freeze()      {}
func (l liveUpdateSyncStep) Truth() starlark.Bool {
	return len(l.localPath) > 0 || len(l.remotePath) > 0
}
func (l liveUpdateSyncStep) Hash() (uint32, error) {
	return starlark.Tuple{starlark.String(l.localPath), starlark.String(l.remotePath)}.Hash()
}
func (l liveUpdateSyncStep) liveUpdateStep()                      {}
func (l liveUpdateSyncStep) declarationPosition() syntax.Position { return l.position }

type liveUpdateRunStep struct {
	command  string
	triggers []string
	position syntax.Position
}

var _ starlark.Value = liveUpdateRunStep{}
var _ liveUpdateStep = liveUpdateRunStep{}

func (l liveUpdateRunStep) String() string {
	return fmt.Sprintf("command: %s ; triggers: %v", strconv.Quote(l.command), l.triggers)
}
func (l liveUpdateRunStep) Type() string { return "live_update_run_step" }
func (l liveUpdateRunStep) Freeze()      {}
func (l liveUpdateRunStep) Truth() starlark.Bool {
	return len(l.command) > 0
}
func (l liveUpdateRunStep) Hash() (uint32, error) {
	t := starlark.Tuple{starlark.String(l.command)}
	for _, trigger := range l.triggers {
		t = append(t, starlark.String(trigger))
	}
	return t.Hash()
}
func (l liveUpdateRunStep) declarationPosition() syntax.Position { return l.position }

func (l liveUpdateRunStep) liveUpdateStep() {}

type liveUpdateRestartContainerStep struct {
	position syntax.Position
}

var _ starlark.Value = liveUpdateRestartContainerStep{}
var _ liveUpdateStep = liveUpdateRestartContainerStep{}

func (l liveUpdateRestartContainerStep) String() string                       { return "restart_container" }
func (l liveUpdateRestartContainerStep) Type() string                         { return "live_update_restart_container_step" }
func (l liveUpdateRestartContainerStep) Freeze()                              {}
func (l liveUpdateRestartContainerStep) Truth() starlark.Bool                 { return true }
func (l liveUpdateRestartContainerStep) Hash() (uint32, error)                { return 0, nil }
func (l liveUpdateRestartContainerStep) declarationPosition() syntax.Position { return l.position }
func (l liveUpdateRestartContainerStep) liveUpdateStep()                      {}

func (s *tiltfileState) liveUpdateWorkDir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var workDir string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "dir", &workDir); err != nil {
		return nil, err
	}

	if !filepath.IsAbs(workDir) {
		return starlark.None, fmt.Errorf("work_dir path '%s' is not absolute", workDir)
	}

	ret := liveUpdateWorkDirStep{
		value:    workDir,
		position: thread.TopFrame().Position(),
	}
	s.unconsumedLiveUpdateSteps = append(s.unconsumedLiveUpdateSteps, ret)
	return ret, nil
}

func (s *tiltfileState) liveUpdateSync(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var localPath, remotePath string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "local_path", &localPath, "remote_path", &remotePath); err != nil {
		return nil, err
	}

	ret := liveUpdateSyncStep{
		localPath:  s.absPath(localPath),
		remotePath: remotePath,
		position:   thread.TopFrame().Position(),
	}
	s.unconsumedLiveUpdateSteps = append(s.unconsumedLiveUpdateSteps, ret)
	return ret, nil
}

func (s *tiltfileState) liveUpdateRun(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var command string
	var triggers starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &command, "trigger?", &triggers); err != nil {
		return nil, err
	}

	triggersSlice := starlarkValueOrSequenceToSlice(triggers)
	var triggerStrings []string
	for _, t := range triggersSlice {
		switch t2 := t.(type) {
		case starlark.String:
			triggerStrings = append(triggerStrings, s.absPath(string(t2)))
		default:
			return nil, fmt.Errorf("run cmd '%s' triggers contained value '%s' of type '%s'. it may only contain strings", command, t.String(), t.Type())
		}
	}

	ret := liveUpdateRunStep{
		command:  command,
		triggers: triggerStrings,
		position: thread.TopFrame().Position(),
	}
	s.unconsumedLiveUpdateSteps = append(s.unconsumedLiveUpdateSteps, ret)
	return ret, nil
}

func (s *tiltfileState) liveUpdateRestartContainer(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}

	ret := liveUpdateRestartContainerStep{
		position: thread.TopFrame().Position(),
	}
	s.unconsumedLiveUpdateSteps = append(s.unconsumedLiveUpdateSteps, ret)
	return ret, nil
}

func liveUpdateStepToModel(l liveUpdateStep) (model.LiveUpdateStep, error) {
	switch x := l.(type) {
	case liveUpdateWorkDirStep:
		return model.LiveUpdateWorkDirStep(x.value), nil
	case liveUpdateSyncStep:
		return model.LiveUpdateSyncStep{Source: x.localPath, Dest: x.remotePath}, nil
	case liveUpdateRunStep:
		return model.LiveUpdateRunStep{
			Command:  model.ToShellCmd(x.command),
			Triggers: x.triggers,
		}, nil
	case liveUpdateRestartContainerStep:
		return model.LiveUpdateRestartContainerStep{}, nil
	default:
		return nil, fmt.Errorf("internal error - unknown liveUpdateStep '%v' of type '%T', declared at %s", l, l, l.declarationPosition().String())
	}
}

func liveUpdateToModel(l liveUpdateDef) (model.LiveUpdate, error) {
	return model.NewLiveUpdate(l.steps, l.fullRebuildTriggers)
}

func (s *tiltfileState) consumeLiveUpdateStep(stepToConsume liveUpdateStep) {
	n := []liveUpdateStep{}
	for i, step := range s.unconsumedLiveUpdateSteps {
		if step.declarationPosition() != stepToConsume.declarationPosition() {
			n = append(n, step)
		} else {
			if i < len(s.unconsumedLiveUpdateSteps)-1 {
				n = append(n, s.unconsumedLiveUpdateSteps[i+1:]...)
			}
			break
		}
	}
	s.unconsumedLiveUpdateSteps = n
}

func (s *tiltfileState) liveUpdate(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dockerRef string
	var steps, fullRebuildTriggers starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs,
		"image", &dockerRef,
		"steps", &steps,
		"full_rebuild_triggers?", &fullRebuildTriggers,
	); err != nil {
		return nil, err
	}

	var modelSteps []model.LiveUpdateStep
	stepSlice := starlarkValueOrSequenceToSlice(steps)
	hasWorkdir := false
	for _, v := range stepSlice {
		step, ok := v.(liveUpdateStep)
		if !ok {
			return starlark.None, fmt.Errorf("'steps' must be a list of live update steps - got value '%v' of type '%s'", v.String(), v.Type())
		}
		switch x := v.(type) {
		case liveUpdateSyncStep:
			if !hasWorkdir {
				if !filepath.IsAbs(x.remotePath) {
					return starlark.None, fmt.Errorf("step '%s' must either follow a work_dir or have an absolute remote_path", x.String())
				}
			}
		case liveUpdateWorkDirStep:
			hasWorkdir = true
		}
		ms, err := liveUpdateStepToModel(step)
		if err != nil {
			return starlark.None, err
		}
		s.consumeLiveUpdateStep(step)
		modelSteps = append(modelSteps, ms)
	}

	frtSlice := starlarkValueOrSequenceToSlice(fullRebuildTriggers)
	var frtStrings []string
	for _, v := range frtSlice {
		str, ok := v.(starlark.String)
		if !ok {
			return starlark.None, fmt.Errorf("'full_rebuild_triggers' must only contain strings - got value '%v' of type '%s'", v.String(), v.Type())
		}
		frtStrings = append(frtStrings, s.absPath(string(str)))
	}

	s.liveUpdates[dockerRef] = &liveUpdateDef{
		steps:               modelSteps,
		fullRebuildTriggers: frtStrings,
	}

	return starlark.None, nil
}

func (s *tiltfileState) validateLiveUpdates() error {
	if len(s.unconsumedLiveUpdateSteps) > 0 {
		var errorStrings []string
		for _, step := range s.unconsumedLiveUpdateSteps {
			errorStrings = append(errorStrings, fmt.Sprintf("value '%s' of type '%s' declared at %s", step.String(), step.Type(), step.declarationPosition().String()))
		}
		return fmt.Errorf("live_update steps were created that were not used by any live_update: %s", strings.Join(errorStrings, ", "))
	}

	for k, v := range s.liveUpdates {
		if !v.matched {
			return fmt.Errorf("live_update was specified for '%s', but no built resource uses that image", k)
		}
	}

	return nil
}
