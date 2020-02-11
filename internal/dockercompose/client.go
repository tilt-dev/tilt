package dockercompose

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type DockerComposeClient interface {
	Up(ctx context.Context, configPaths []string, serviceName model.TargetName, shouldBuild bool, stdout, stderr io.Writer) error
	Down(ctx context.Context, configPaths []string, stdout, stderr io.Writer) error
	StreamLogs(ctx context.Context, configPaths []string, serviceName model.TargetName) (io.ReadCloser, error)
	StreamEvents(ctx context.Context, configPaths []string) (<-chan string, error)
	Config(ctx context.Context, configPaths []string) (string, error)
	Services(ctx context.Context, configPaths []string) (string, error)
	ContainerID(ctx context.Context, configPaths []string, serviceName model.TargetName) (container.ID, error)
}

type cmdDCClient struct {
	env docker.Env
	mu  *sync.Mutex
}

// TODO(dmiller): we might want to make this take a path to the docker-compose config so we don't
// have to keep passing it in.
func NewDockerComposeClient(env docker.LocalEnv) DockerComposeClient {
	return &cmdDCClient{
		env: docker.Env(env),
		mu:  &sync.Mutex{},
	}
}

func (c *cmdDCClient) Up(ctx context.Context, configPaths []string, serviceName model.TargetName, shouldBuild bool, stdout, stderr io.Writer) error {
	// docker-compose up is not thread-safe, because network operations are non-atomic. See:
	// https://github.com/windmilleng/tilt/issues/2817
	c.mu.Lock()
	defer c.mu.Unlock()

	var args []string
	if logger.Get(ctx).Level().ShouldDisplay(logger.VerboseLvl) {
		args = []string{"--verbose"}
	}

	for _, config := range configPaths {
		args = append(args, "-f", config)
	}

	args = append(args, "up", "--no-deps", "-d")

	if shouldBuild {
		args = append(args, "--build")
	} else {
		// !shouldBuild implies that Tilt will take care of building, which implies that
		// we should recreate container so that we pull the new image
		// NOTE(maia): this is maybe the WRONG thing to do if we're deploying a service
		// but none of the code changed (i.e. it was just a dockercompose.yml change)?
		args = append(args, "--force-recreate")
	}

	args = append(args, serviceName.String())
	cmd := c.dcCommand(ctx, args)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return FormatError(cmd, nil, cmd.Run())
}

func (c *cmdDCClient) Down(ctx context.Context, configPaths []string, stdout, stderr io.Writer) error {
	// To be safe, we try not to run two docker-compose downs in parallel,
	// because we know docker-compose up is not thread-safe.
	c.mu.Lock()
	defer c.mu.Unlock()

	var args []string
	if logger.Get(ctx).Level().ShouldDisplay(logger.VerboseLvl) {
		args = []string{"--verbose"}
	}
	for _, config := range configPaths {
		args = append(args, "-f", config)
	}

	args = append(args, "down")
	cmd := c.dcCommand(ctx, args)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil {
		return FormatError(cmd, nil, err)
	}

	return nil
}

func (c *cmdDCClient) StreamLogs(ctx context.Context, configPaths []string, serviceName model.TargetName) (io.ReadCloser, error) {
	// TODO(maia): --since time
	// (may need to implement with `docker log <cID>` instead since `d-c log` doesn't support `--since`
	var args []string
	for _, config := range configPaths {
		args = append(args, "-f", config)
	}
	args = append(args, "logs", "--no-color", "-f", serviceName.String())
	cmd := c.dcCommand(ctx, args)
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

func (c *cmdDCClient) StreamEvents(ctx context.Context, configPaths []string) (<-chan string, error) {
	ch := make(chan string)

	var args []string
	for _, config := range configPaths {
		args = append(args, "-f", config)
	}
	args = append(args, "events", "--json")
	cmd := c.dcCommand(ctx, args)
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

func (c *cmdDCClient) Config(ctx context.Context, configPaths []string) (string, error) {
	return c.dcOutput(ctx, configPaths, "config")
}

func (c *cmdDCClient) Services(ctx context.Context, configPaths []string) (string, error) {
	return c.dcOutput(ctx, configPaths, "config", "--services")
}

func (c *cmdDCClient) ContainerID(ctx context.Context, configPaths []string, serviceName model.TargetName) (container.ID, error) {
	id, err := c.dcOutput(ctx, configPaths, "ps", "-q", serviceName.String())
	if err != nil {
		return container.ID(""), err
	}

	return container.ID(id), nil
}

func (c *cmdDCClient) dcCommand(ctx context.Context, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "docker-compose", args...)
	cmd.Env = append(os.Environ(), c.env.AsEnviron()...)
	return cmd
}

func (c *cmdDCClient) dcOutput(ctx context.Context, configPaths []string, args ...string) (string, error) {

	var tempArgs []string
	for _, config := range configPaths {
		tempArgs = append(tempArgs, "-f", config)
	}
	args = append(tempArgs, args...)
	cmd := c.dcCommand(ctx, args)

	output, err := cmd.Output()
	if err != nil {
		errorMessage := fmt.Sprintf("command %q failed.\nerror: %v\nstdout: %q", cmd.Args, err, string(output))
		if err, ok := err.(*exec.ExitError); ok {
			errorMessage += fmt.Sprintf("\nstderr: '%v'", string(err.Stderr))
		}
		err = fmt.Errorf(errorMessage)
	}
	return strings.TrimSpace(string(output)), err
}

func FormatError(cmd *exec.Cmd, stdout []byte, err error) error {
	if err == nil {
		return nil
	}
	errorMessage := fmt.Sprintf("command %q failed.\nerror: %v\n", cmd.Args, err)
	if len(stdout) > 0 {
		errorMessage += fmt.Sprintf("\nstdout: '%v'", string(stdout))
	}
	if err, ok := err.(*exec.ExitError); ok && len(err.Stderr) > 0 {
		errorMessage += fmt.Sprintf("\nstderr: '%v'", string(err.Stderr))
	}
	return fmt.Errorf(errorMessage)
}
