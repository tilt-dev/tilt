package build

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/model"
)

// https://success.docker.com/article/what-causes-a-container-to-exit-with-code-137
const TaskKillExitCode = 137

// Convert a Docker exec error into our own internal error type.
func WrapContainerExecError(err error, cID container.ID, cmd model.Cmd) error {
	exitErr, isExitErr := err.(docker.ExitError)
	if isExitErr {
		if exitErr.ExitCode == TaskKillExitCode {
			// If we got a 137 error code, that's not the user's fault.
			// The k8s infrastructure killed the job.
			return fmt.Errorf("executing %v on container %s: killed by container engine", cmd, cID.ShortStr())
		}

		return RunStepFailure{ExitCode: exitErr.ExitCode}
	}

	return errors.Wrapf(err, "executing %v on container %s", cmd, cID.ShortStr())
}

// Indicates that the update failed because one of the user's Runs failed
// (i.e. exited non-zero) -- as opposed to an infrastructure issue.
type RunStepFailure struct {
	ExitCode int
}

func (e RunStepFailure) Error() string {
	return fmt.Sprintf("Run step failed with exit code: %d", e.ExitCode)
}

func IsRunStepFailure(err error) bool {
	_, ok := err.(RunStepFailure)
	return ok
}

var _ error = RunStepFailure{}
