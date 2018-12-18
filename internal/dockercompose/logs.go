package dockercompose

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/logger"
)

func LogReaderForService(ctx context.Context, svcName, configPath string) (io.ReadCloser, error) {
	// TODO(maia): --since time
	// (may need to implement with `docker log <cID>` instead since `d-c log` doesn't support `--since`
	args := []string{"-f", configPath, "logs", "-f", "-t", svcName} // TODO(maia): probs don't need -t
	cmd := exec.CommandContext(ctx, "docker-compose", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "making stdout pipe for `docker-compose logs`")
	}

	errBuf := bytes.Buffer{}
	cmd.Stderr = &errBuf

	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrapf(err, "`docker-compose %s`",
			strings.Join(args, " "))
	}

	go func() {
		err = cmd.Wait()
		if err != nil {
			logger.Get(ctx).Debugf("cmd `docker-compose %s` exited with error: \"%v\" (stderr: %s)",
				strings.Join(args, " "), err, errBuf.String())
		}
	}()
	return stdout, nil
}
