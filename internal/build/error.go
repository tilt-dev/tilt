package build

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/exec"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/pkg/model"
)

// https://success.docker.com/article/what-causes-a-container-to-exit-with-code-137
const TaskKillExitCode = 137

func WrapCodeExitError(err error, cID container.ID, cmd model.Cmd) error {
	exitErr, isExitErr := err.(exec.CodeExitError)
	if isExitErr {
		return RunStepFailure{
			Cmd:      cmd,
			ExitCode: exitErr.ExitStatus(),
		}
	}
	return errors.Wrapf(err, "executing %v on container %s", cmd, cID.ShortStr())
}

// Convert a Docker exec error into our own internal error type.
func WrapContainerExecError(err error, cID container.ID, cmd model.Cmd) error {
	exitErr, isExitErr := err.(docker.ExitError)
	if isExitErr {
		if exitErr.ExitCode == TaskKillExitCode {
			// If we got a 137 error code, that's not the user's fault.
			// The k8s infrastructure killed the job.
			return fmt.Errorf("executing %v on container %s: killed by container engine", cmd, cID.ShortStr())
		}

		return RunStepFailure{
			Cmd:      cmd,
			ExitCode: exitErr.ExitCode,
		}
	}

	return errors.Wrapf(err, "executing %v on container %s", cmd, cID.ShortStr())
}

// Indicates that the update failed because one of the user's Runs failed
// (i.e. exited non-zero) -- as opposed to an infrastructure issue.
type RunStepFailure struct {
	Cmd      model.Cmd
	ExitCode int
}

func (e RunStepFailure) Empty() bool {
	return e.Cmd.Empty() && e.ExitCode == 0
}

func (e RunStepFailure) Error() string {
	return fmt.Sprintf("Run step %q failed with exit code: %d", e.Cmd.String(), e.ExitCode)
}

func IsRunStepFailure(err error) bool {
	_, ok := MaybeRunStepFailure(err)
	return ok
}

func MaybeRunStepFailure(err error) (RunStepFailure, bool) {
	e := err
	for {
		if e == nil {
			break
		}
		rsf, ok := e.(RunStepFailure)
		if ok {
			return rsf, true
		}
		cause := errors.Cause(e)
		if cause == e {
			// no more causes to drill into
			// (If err does not implement Causer, `Cause(err)` returns back the original error)
			break
		}
		e = cause
	}
	return RunStepFailure{}, false
}

var _ error = RunStepFailure{}
