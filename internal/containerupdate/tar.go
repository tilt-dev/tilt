package containerupdate

import (
	"fmt"

	"github.com/tilt-dev/tilt/pkg/model"
)

const (
	// TarExitCodePermissionDenied is returned by `tar` if it does not have
	// sufficient permissions to write the extracted files.
	TarExitCodePermissionDenied = 2
	// GenericExitCodeCannotExec indicates the command cannot be executed.
	// In a shell, this generally is a form of permission issues (i.e. the
	// binary was found but is not +x). However, container runtimes also
	// use this to indicate that the binary wasn't found at all, which is
	// extremely common when we try to use common tools such as `tar` but
	// the image is missing them.
	GenericExitCodeCannotExec = 126
)

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
		"Some minimal images, such as those that are built `FROM scratch`,"+
		"will not work with Live Update by default.\n"+
		"See https://github.com/tilt-dev/tilt/issues/4303 for details.",
		err)
}
