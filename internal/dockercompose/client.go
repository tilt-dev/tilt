package dockercompose

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/compose-spec/compose-go/loader"

	"github.com/compose-spec/compose-go/types"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	compose "github.com/compose-spec/compose-go/cli"
)

type DockerComposeClient interface {
	Up(ctx context.Context, configPaths []string, serviceName model.TargetName, shouldBuild bool, stdout, stderr io.Writer) error
	Down(ctx context.Context, configPaths []string, stdout, stderr io.Writer) error
	StreamLogs(ctx context.Context, configPaths []string, serviceName model.TargetName) io.ReadCloser
	StreamEvents(ctx context.Context, configPaths []string) (<-chan string, error)
	Project(ctx context.Context, configPaths []string) (*types.Project, error)
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
	var genArgs []string
	// TODO(milas): this causes docker-compose to output a truly excessive amount of logging; it might
	// 	make sense to hide it behind a special environment variable instead or something
	if logger.Get(ctx).Level().ShouldDisplay(logger.VerboseLvl) {
		genArgs = []string{"--verbose"}
	}

	for _, config := range configPaths {
		genArgs = append(genArgs, "-f", config)
	}

	if shouldBuild {
		var buildArgs = append([]string{}, genArgs...)
		buildArgs = append(buildArgs, "build", serviceName.String())
		cmd := c.dcCommand(ctx, buildArgs)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Run()
		if err != nil {
			return FormatError(cmd, nil, err)
		}
	}

	// docker-compose up is not thread-safe, because network operations are non-atomic. See:
	// https://github.com/tilt-dev/tilt/issues/2817
	//
	// docker-compose build can run in parallel fine, so we only want the mutex on the 'up' call.
	//
	// TODO(nick): It might make sense to use a CondVar so that we can log a message
	// when we're waiting on another build...
	c.mu.Lock()
	defer c.mu.Unlock()
	runArgs := append([]string{}, genArgs...)
	runArgs = append(runArgs, "up", "--no-deps", "--no-build", "-d")

	runArgs = append(runArgs, serviceName.String())
	cmd := c.dcCommand(ctx, runArgs)
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

func (c *cmdDCClient) StreamLogs(ctx context.Context, configPaths []string, serviceName model.TargetName) io.ReadCloser {
	var args []string
	for _, config := range configPaths {
		args = append(args, "-f", config)
	}

	r, w := io.Pipe()

	// NOTE: we can't practically remove "--no-color" due to the way that Docker Compose formats colorful lines; it
	// 		 will wrap the entire line (including the \n) with the color codes, so you end up with something like:
	//			\u001b[36mmyproject_my-container_1 exited with code 0\n\u001b[0m
	// 		 where the ANSI reset (\u001b[0m) is _AFTER_ the \n, which doesn't play nice with our log segment logic
	// 		 under some conditions - adding a final \n after stdout is closed would probably be sufficient given the
	// 		 current pattern of how Compose colorizes stuff, but it's really not worth the headache to find out
	args = append(args, "logs", "--no-color", "--no-log-prefix", "--timestamps", "--follow", serviceName.String())
	cmd := c.dcCommand(ctx, args)
	cmd.Stdout = w

	errBuf := bytes.Buffer{}
	cmd.Stderr = &errBuf

	go func() {
		if cmdErr := cmd.Run(); cmdErr != nil {
			_ = w.CloseWithError(fmt.Errorf("cmd `docker-compose %s` exited with error: \"%v\" (stderr: %s)",
				strings.Join(args, " "), cmdErr, errBuf.String()))
		} else {
			_ = w.Close()
		}
	}()
	return r
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

func (c *cmdDCClient) Project(ctx context.Context, configPaths []string) (*types.Project, error) {
	proj, err := c.loadProject(configPaths)
	if err == nil {
		return proj, nil
	}

	// HACK(milas): compose-go has known regressions with resolving variables during YAML loading
	// 	if it fails, attempt to fallback to using the CLI to resolve the YAML and then parse
	// 	it with compose-go
	// 	see https://github.com/tilt-dev/tilt/issues/4795
	//var fallbackErr error
	proj, err = c.loadProjectCLI(ctx, configPaths)
	if err != nil {
		return nil, err
	}
	return proj, nil
}

func (c *cmdDCClient) ContainerID(ctx context.Context, configPaths []string, serviceName model.TargetName) (container.ID, error) {
	id, err := c.dcOutput(ctx, configPaths, "ps", "-q", serviceName.String())
	if err != nil {
		return container.ID(""), err
	}

	return container.ID(id), nil
}

func (c *cmdDCClient) loadProject(configPaths []string) (*types.Project, error) {
	opts, err := compose.NewProjectOptions(configPaths, compose.WithOsEnv)
	if err != nil {
		return nil, err
	}
	proj, err := compose.ProjectFromOptions(opts)
	if err != nil {
		return nil, err
	}
	return proj, nil
}

func (c *cmdDCClient) loadProjectCLI(ctx context.Context, configPaths []string) (*types.Project, error) {
	resolvedYAML, err := c.dcOutput(ctx, configPaths, "config")
	if err != nil {
		return nil, err
	}

	// in practice, the workdir should be irrelevant as the CLI call above _should_ already have resolved paths,
	// but we populate it appropriately regardless because historically docker-compose has been inconsistent in
	// this regard
	var workDir string
	if len(configPaths) != 0 {
		// from the compose Docs:
		// 	> When you use multiple Compose files, all paths in the files are relative to the first configuration file specified with -f
		// https://docs.docker.com/compose/reference/#use--f-to-specify-name-and-path-of-one-or-more-compose-files
		workDir = filepath.Dir(configPaths[0])
	}

	return loader.Load(types.ConfigDetails{
		WorkingDir: workDir,
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(resolvedYAML),
			},
		},
	})
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
		err = fmt.Errorf("%s", errorMessage)
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
	return fmt.Errorf("%s", errorMessage)
}
