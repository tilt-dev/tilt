package tiltd_server

import (
	"context"
	"io"
	"os"
	"os/exec"
)

func RunDaemon(ctx context.Context, out io.Writer) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, os.Args[0], "daemon")
	cmd.Stdout = out
	cmd.Stderr = out

	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return cmd, nil
}
