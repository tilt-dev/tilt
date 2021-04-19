package browser

import "os/exec"

func openBrowser(url string, cmdOptions []CmdOption) error {
	return runCmd("open", []string{url}, cmdOptions)
}

func setFlags(cmd *exec.Cmd) {}
