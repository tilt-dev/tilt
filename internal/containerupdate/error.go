package containerupdate

import (
	"errors"
	"fmt"

	"k8s.io/client-go/util/exec"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/pkg/model"
)

const (
	// GenericExitCodeKilled indicates the container runtime killed the process.
	GenericExitCodeKilled = 137

	// GenericExitCodeCannotExec indicates the command cannot be executed.
	// In a shell, this generally is a form of permission issues (i.e. the
	// binary was found but is not +x). However, container runtimes also
	// use this to indicate that the binary wasn't found at all, which is
	// extremely common when we try to use common tools such as `tar` but
	// the image is missing them.
	GenericExitCodeCannotExec = 126

	GenericExitCodeNotFound = 127
)

// ExtractExitCode returns exit status information from different types of
// errors.
func ExtractExitCode(err error) (int, bool) {
	var k8sExitErr exec.ExitError
	if errors.As(err, &k8sExitErr) {
		return k8sExitErr.ExitStatus(), true
	}

	var dockerExitErr docker.ExitError
	if errors.As(err, &dockerExitErr) {
		return dockerExitErr.ExitCode, true
	}

	return -1, false
}

type ExecError struct {
	Cmd      model.Cmd
	ExitCode int
}

func NewExecError(cmd model.Cmd, exitCode int) ExecError {
	return ExecError{Cmd: cmd, ExitCode: exitCode}
}

func (e ExecError) Error() string {
	var reason string
	switch e.ExitCode {
	case GenericExitCodeKilled:
		reason = "killed by container runtime"
	case GenericExitCodeCannotExec:
		// docker uses this when it can't find the command for an exec, so we
		// have a single error message that covers both cases
		fallthrough
	case GenericExitCodeNotFound:
		reason = "not found in PATH or not executable"
	}

	if reason != "" {
		reason = " (" + reason + ")"
	}

	return fmt.Sprintf("command %q failed with exit code: %d%s", e.Cmd.String(), e.ExitCode, reason)
}

func wrapRunStepError(err error) error {
	var execErr ExecError
	if errors.As(err, &execErr) {
		if execErr.ExitCode == GenericExitCodeKilled {
			// a SIGKILL of a run() command is not treated as a step failure
			// but as a more generic "infra" error, so it's not wrapped
			return err
		}
		return build.NewRunStepFailure(err)
	}
	return err
}
