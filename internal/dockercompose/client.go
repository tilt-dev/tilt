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

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

type DockerComposeClient interface {
	Up(ctx context.Context, configPath string, serviceName model.TargetName, shouldBuild bool, stdout, stderr io.Writer) error
	Down(ctx context.Context, configPath string, stdout, stderr io.Writer) error
	StreamLogs(ctx context.Context, configPath string, serviceName model.TargetName) (io.ReadCloser, error)
	StreamEvents(ctx context.Context, configPath string) (<-chan string, error)
	Config(ctx context.Context, configPath string) (string, error)
	Services(ctx context.Context, configPath string) (string, error)
	ContainerID(ctx context.Context, configPath string, serviceName model.TargetName) (container.ID, error)
}

type cmdDCClient struct {
	// NOTE(nick): In an ideal world, we would detect if the user was using
	// docker-compose or kubernetes as an orchestration engine, and use that to
	// choose an appropriate docker-client. But the docker-client is wired up
	// at start-time.
	//
	// So for now, we need docker-compose to use the same docker client as
	// everybody else, even if it's a weird docker client (like the docker client
	// that lives in minikube).
	env docker.Env
}

// TODO(dmiller): we might want to make this take a path to the docker-compose config so we don't
// have to keep passing it in.
func NewDockerComposeClient(env docker.Env) DockerComposeClient {
	return &cmdDCClient{
		env: env,
	}
}

func (c *cmdDCClient) Up(ctx context.Context, configPath string, serviceName model.TargetName, shouldBuild bool, stdout, stderr io.Writer) error {
	var args []string
	if logger.Get(ctx).Level() >= logger.VerboseLvl {
		args = []string{"--verbose"}
	}

	args = append(args, "-f", configPath, "up", "--no-deps", "-d")
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

func (c *cmdDCClient) Down(ctx context.Context, configPath string, stdout, stderr io.Writer) error {
	var args []string
	if logger.Get(ctx).Level() >= logger.VerboseLvl {
		args = []string{"--verbose"}
	}
	args = append(args, "-f", configPath, "down")
	cmd := c.dcCommand(ctx, args)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil {
		return FormatError(cmd, nil, err)
	}

	return nil
}

func (c *cmdDCClient) StreamLogs(ctx context.Context, configPath string, serviceName model.TargetName) (io.ReadCloser, error) {
	// TODO(maia): --since time
	// (may need to implement with `docker log <cID>` instead since `d-c log` doesn't support `--since`
	args := []string{"-f", configPath, "logs", "-f", "-t", serviceName.String()}
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

func (c *cmdDCClient) StreamEvents(ctx context.Context, configPath string) (<-chan string, error) {
	ch := make(chan string)

	args := []string{"-f", configPath, "events", "--json"}
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

func (c *cmdDCClient) Config(ctx context.Context, configPath string) (string, error) {
	return c.dcOutput(ctx, configPath, "config")
}

func (c *cmdDCClient) Services(ctx context.Context, configPath string) (string, error) {
	return c.dcOutput(ctx, configPath, "config", "--services")
}

func (c *cmdDCClient) ContainerID(ctx context.Context, configPath string, serviceName model.TargetName) (container.ID, error) {
	id, err := c.dcOutput(ctx, configPath, "ps", "-q", serviceName.String())
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

func (c *cmdDCClient) dcOutput(ctx context.Context, configPath string, args ...string) (string, error) {
	args = append([]string{"-f", configPath}, args...)
	cmd := c.dcCommand(ctx, args)

	output, err := cmd.Output()
	if err != nil {
		errorMessage := fmt.Sprintf("command 'docker-compose %q' failed.\nerror: '%v'\nstdout: '%v'", args, err, string(output))
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
	errorMessage := fmt.Sprintf("command '%q %q' failed.\nerror: '%v'\n", cmd.Path, cmd.Args, err)
	if len(stdout) > 0 {
		errorMessage += fmt.Sprintf("\nstdout: '%v'", string(stdout))
	}
	if err, ok := err.(*exec.ExitError); ok && len(err.Stderr) > 0 {
		errorMessage += fmt.Sprintf("\nstderr: '%v'", string(err.Stderr))
	}
	return fmt.Errorf(errorMessage)
}
