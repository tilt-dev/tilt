package openurl

import (
	"io"
	"os/exec"

	"github.com/pkg/browser"
)

type OpenURL func(url string, out io.Writer) error

func BrowserOpen(url string, out io.Writer) error {
	return browser.OpenURL(url, func(cmd *exec.Cmd) {
		cmd.Stdout = out
		cmd.Stderr = out
	})
}
