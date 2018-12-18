package dockercompose

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/logger"
)

type DockerComposeClient interface {
	Up(ctx context.Context, pathToConfig, serviceName string, stdout, stderr io.Writer) error
	Down(ctx context.Context, pathToConfig string, stdout, stderr io.Writer) error
	Logs(ctx context.Context, pathToConfig, serviceName string) (io.ReadCloser, error)
	Events(ctx context.Context, pathToConfig string) (<-chan string, error)
	Config(ctx context.Context, pathToConfig string) (string, error)
	Services(ctx context.Context, pathToConfig string) (string, error)
}

type cmdDCClient struct{}

func NewDockerComposeClient() DockerComposeClient {
	return &cmdDCClient{}
}

func (c *cmdDCClient) Up(ctx context.Context, pathToConfig, serviceName string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "docker-compose", "-f", pathToConfig, "up", "--no-deps", "--build", "--force-recreate", "-d", serviceName)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	return FormatError(cmd, nil, err)
}

func (c *cmdDCClient) Down(ctx context.Context, pathToConfig string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "docker-compose", "-f", pathToConfig, "down")
	cmd.Stdout = logger.Get(ctx).Writer(logger.InfoLvl)
	cmd.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)

	err := cmd.Run()
	if err != nil {
		return FormatError(cmd, nil, err)
	}

	return nil
}

func (c *cmdDCClient) Logs(ctx context.Context, pathToConfig, serviceName string) (io.ReadCloser, error) {
	// TODO(maia): --since time
	// (may need to implement with `docker log <cID>` instead since `d-c log` doesn't support `--since`
	args := []string{"-f", pathToConfig, "logs", "-f", "-t", serviceName} // ~~ don't need -t probs
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

func (c *cmdDCClient) Events(ctx context.Context, pathToConfig string) (<-chan string, error) {
	ch := make(chan string)

	args := []string{"-f", pathToConfig, "events", "--json"}
	cmd := exec.CommandContext(ctx, "docker-compose", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ch, errors.Wrap(err, "making stdout pipe for `docker-compose events`")
	}

	err = cmd.Start()
	if err != nil {
		return ch, errors.Wrapf(err, "`docker-compose %s`",
			strings.Join(args, " "))
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			ch <- scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			logger.Get(ctx).Infof("[DOCKER-COMPOSE WATCHER] scanning `events` output: %v", err)
		}

		err = cmd.Wait()
		if err != nil {
			logger.Get(ctx).Infof("[DOCKER-COMPOSE WATCHER] exited with error: %v", err)
		}
	}()

	return ch, nil
}

func (c *cmdDCClient) Config(ctx context.Context, pathToConfig string) (string, error) {
	return dcOutput(ctx, pathToConfig, "config")
}

func (c *cmdDCClient) Services(ctx context.Context, pathToConfig string) (string, error) {
	return dcOutput(ctx, pathToConfig, "config", "--services")
}

func dcOutput(ctx context.Context, configPath string, args ...string) (string, error) {
	args = append([]string{"-f", configPath}, args...)
	output, err := exec.CommandContext(ctx, "docker-compose", args...).Output()
	if err != nil {
		errorMessage := fmt.Sprintf("command 'docker-compose %q' failed.\nerror: '%v'\nstdout: '%v'", args, err, string(output))
		if err, ok := err.(*exec.ExitError); ok {
			errorMessage += fmt.Sprintf("\nstderr: '%v'", string(err.Stderr))
		}
		err = fmt.Errorf(errorMessage)
	}
	return string(output), err
}
