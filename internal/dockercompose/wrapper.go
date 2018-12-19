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
	Up(ctx context.Context, configPath, serviceName string, stdout, stderr io.Writer) error
	Down(ctx context.Context, configPath string, stdout, stderr io.Writer) error
	Logs(ctx context.Context, configPath, serviceName string) (io.ReadCloser, error)
	Events(ctx context.Context, configPath string) (<-chan string, error)
	Config(ctx context.Context, configPath string) (string, error)
	Services(ctx context.Context, configPath string) (string, error)
}

type cmdDCClient struct{}

// TODO(dmiller): we might want to make this take a path to the docker-compose config so we don't
// have to keep passing it in.
func NewDockerComposeClient() DockerComposeClient {
	return &cmdDCClient{}
}

func (c *cmdDCClient) Up(ctx context.Context, configPath, serviceName string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "docker-compose", "-f", configPath, "up", "--no-deps", "--build", "--force-recreate", "-d", serviceName)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return FormatError(cmd, nil, cmd.Run())
}

func (c *cmdDCClient) Down(ctx context.Context, configPath string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "docker-compose", "-f", configPath, "down")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil {
		return FormatError(cmd, nil, err)
	}

	return nil
}

// TODO(dmiller) rename to indicate that this streams logs to the io.ReadCloser
func (c *cmdDCClient) Logs(ctx context.Context, configPath, serviceName string) (io.ReadCloser, error) {
	// TODO(maia): --since time
	// (may need to implement with `docker log <cID>` instead since `d-c log` doesn't support `--since`
	args := []string{"-f", configPath, "logs", "-f", "-t", serviceName} // ~~ don't need -t probs
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

// TODO(dmiller) rename to indicate that this streams logs to the chan string
func (c *cmdDCClient) Events(ctx context.Context, configPath string) (<-chan string, error) {
	ch := make(chan string)

	args := []string{"-f", configPath, "events", "--json"}
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

func (c *cmdDCClient) Config(ctx context.Context, configPath string) (string, error) {
	return dcOutput(ctx, configPath, "config")
}

func (c *cmdDCClient) Services(ctx context.Context, configPath string) (string, error) {
	return dcOutput(ctx, configPath, "config", "--services")
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
