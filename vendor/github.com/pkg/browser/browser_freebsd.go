package browser

import (
	"errors"
	"os/exec"
)

func openBrowser(url string, cmdOptions []CmdOption) error {
	err := runCmd("xdg-open", []string{url}, cmdOptions)
	if e, ok := err.(*exec.Error); ok && e.Err == exec.ErrNotFound {
		return errors.New("xdg-open: command not found - install xdg-utils from ports(8)")
	}
	return err
}

func setFlags(cmd *exec.Cmd) {}
