package dockercompose

import (
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func LogReaderForService(ctx context.Context, svcName, configPath string) (io.ReadCloser, error) {
	// TODO(maia): --since time
	// (may need to implement with `docker log <cID>` instead since `d-c log` doesn't support `--since`
	args := []string{"-f", configPath, "-f", "-t", "logs", svcName} // ~~ don't need -t probs
	cmd := exec.CommandContext(ctx, "docker-compose", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "making stdout pipe for `docker-compose logs`")
	}

	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrapf(err, "`docker-compose %s`",
			strings.Join(args, " "))
	}

	return stdout, nil
}
