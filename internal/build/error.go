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

		return UserBuildFailure{ExitCode: exitErr.ExitCode}
	}

	return errors.Wrapf(err, "executing %v on container %s", cmd, cID.ShortStr())
}

// Indicates that the build failed because the user script failed
// (as opposed to an infrastructure issue).
type UserBuildFailure struct {
	ExitCode int
}

func (e UserBuildFailure) Error() string {
	return fmt.Sprintf("Command failed with exit code: %d", e.ExitCode)
}

func IsUserBuildFailure(err error) bool {
	_, ok := err.(UserBuildFailure)
	return ok
}

var _ error = UserBuildFailure{}
