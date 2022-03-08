package containerupdate

import (
	"fmt"

	"github.com/tilt-dev/tilt/pkg/model"
)

// TarExitCodePermissionDenied is returned by `tar` if it does not have
// sufficient permissions to write the extracted files.
const TarExitCodePermissionDenied = 2

func tarCmd() model.Cmd {
	return model.Cmd{
		Argv: []string{"tar", "-C", "/", "-x", "-f", "-"},
	}
}

func permissionDeniedErr(err error) error {
	return fmt.Errorf("%v\n"+
		"This usually means the container filesystem denied access. Please check:\n"+
		"  1) That the container image has writable files\n"+
		"  2) That the container image default user has write access to the files\n"+
		"  3) That the Pod spec doesn't have a SecurityContext that would block writes",
		err)
}

func cannotExecErr(err error) error {
	return fmt.Errorf("%v\n"+
		"This usually means that Tilt could not exec `tar`.\n"+
		"Please check that the container image includes `tar` in $PATH.\n"+
		"Some minimal images, such as those that are built `FROM scratch`, "+
		"will not work with Live Update by default.\n"+
		"See https://github.com/tilt-dev/tilt/issues/4303 for details.",
		err)
}

func wrapTarExecErr(err error, cmd model.Cmd, exitCode int) error {
	switch exitCode {
	case TarExitCodePermissionDenied:
		return permissionDeniedErr(err)
	case GenericExitCodeCannotExec:
		// docker uses this to mean not found, so it's treated the same
		fallthrough
	case GenericExitCodeNotFound:
		// in a `run()` step or other user-defined command, this typically
		// is handled by build.RunStepFailure with a fairly generic message
		// about not found in PATH, but here we provide Live Update specific
		// guidance, since the user will need to adjust their image
		return cannotExecErr(err)
	default:
		return NewExecError(cmd, exitCode)
	}
}
