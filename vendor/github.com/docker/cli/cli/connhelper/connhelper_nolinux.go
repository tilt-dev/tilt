// +build !linux

package connhelper

import (
	"os/exec"
)

func setPdeathsig(cmd *exec.Cmd) {
}
