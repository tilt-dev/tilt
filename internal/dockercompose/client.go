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
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/compose-spec/compose-go/loader"
	"golang.org/x/mod/semver"

	"github.com/compose-spec/compose-go/types"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	compose "github.com/compose-spec/compose-go/cli"
)

// versionRegex handles both v1 and v2 version outputs, which have several variations.
// (See TestParseComposeVersionOutput for various cases.)
var versionRegex = regexp.MustCompile(`(?mi)^docker[ -]compose(?: version)?:? v?([^\s,]+),?(?: build ([a-z0-9-]+))?`)

// dcProjectOptions are used when loading Docker Compose projects via the Go library.
//
// See also: dcLoaderOption which is used for loading projects from the CLI fallback and for tests, which should
// be kept in sync behavior-wise.
var dcProjectOptions = []compose.ProjectOptionsFn{
	compose.WithResolvedPaths(true),
	compose.WithNormalization(true),
	compose.WithOsEnv,
	compose.WithDotEnv,
}

type DockerComposeClient interface {
	Up(ctx context.Context, spec model.DockerComposeUpSpec, shouldBuild bool, stdout, stderr io.Writer) error
	Down(ctx context.Context, p model.DockerComposeProject, stdout, stderr io.Writer) error
	Rm(ctx context.Context, specs []model.DockerComposeUpSpec, stdout, stderr io.Writer) error
	StreamLogs(ctx context.Context, spec model.DockerComposeUpSpec) io.ReadCloser
	StreamEvents(ctx context.Context, spec model.DockerComposeProject) (<-chan string, error)
	Project(ctx context.Context, spec model.DockerComposeProject) (*types.Project, error)
	ContainerID(ctx context.Context, spec model.DockerComposeUpSpec) (container.ID, error)
	Version(ctx context.Context) (canonicalVersion string, build string, err error)
}

type cmdDCClient struct {
	env         docker.Env
	mu          *sync.Mutex
	composePath string
}

// TODO(dmiller): we might want to make this take a path to the docker-compose config so we don't
// have to keep passing it in.
func NewDockerComposeClient(env docker.LocalEnv) DockerComposeClient {
	return &cmdDCClient{
		env:         docker.Env(env),
		mu:          &sync.Mutex{},
		composePath: dcExecutablePath(),
	}
}

func (c *cmdDCClient) projectArgs(p model.DockerComposeProject) []string {
	if p.YAML != "" {
		args := []string{"-f", "-"}
		if p.ProjectPath != "" {
			args = append(args, "--project-directory", p.ProjectPath)
		}
		return args
	}
	result := []string{}
	for _, cp := range p.ConfigPaths {
		result = append(result, "-f", cp)
	}
	return result
}

func (c *cmdDCClient) Up(ctx context.Context, spec model.DockerComposeUpSpec, shouldBuild bool, stdout, stderr io.Writer) error {
	genArgs := c.projectArgs(spec.Project)
	// TODO(milas): this causes docker-compose to output a truly excessive amount of logging; it might
	// 	make sense to hide it behind a special environment variable instead or something
	if logger.Get(ctx).Level().ShouldDisplay(logger.VerboseLvl) {
		genArgs = append(genArgs, "--verbose")
	}

	if shouldBuild {
		var buildArgs = append([]string{}, genArgs...)
		buildArgs = append(buildArgs, "build", spec.Service)
		cmd := c.dcCommand(ctx, buildArgs)
		cmd.Stdin = strings.NewReader(spec.Project.YAML)
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

	runArgs = append(runArgs, spec.Service)
	cmd := c.dcCommand(ctx, runArgs)
	cmd.Stdin = strings.NewReader(spec.Project.YAML)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return FormatError(cmd, nil, cmd.Run())
}

func (c *cmdDCClient) Down(ctx context.Context, p model.DockerComposeProject, stdout, stderr io.Writer) error {
	// To be safe, we try not to run two docker-compose downs in parallel,
	// because we know docker-compose up is not thread-safe.
	c.mu.Lock()
	defer c.mu.Unlock()

	args := c.projectArgs(p)
	if logger.Get(ctx).Level().ShouldDisplay(logger.VerboseLvl) {
		args = append(args, "--verbose")
	}

	args = append(args, "down")
	cmd := c.dcCommand(ctx, args)
	cmd.Stdin = strings.NewReader(p.YAML)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil {
		return FormatError(cmd, nil, err)
	}

	return nil
}

func (c *cmdDCClient) Rm(ctx context.Context, specs []model.DockerComposeUpSpec, stdout, stderr io.Writer) error {
	if len(specs) == 0 {
		return nil
	}

	// To be safe, we try not to run two docker-compose downs in parallel,
	// because we know docker-compose up is not thread-safe.
	c.mu.Lock()
	defer c.mu.Unlock()

	p := specs[0].Project
	args := c.projectArgs(p)
	if logger.Get(ctx).Level().ShouldDisplay(logger.VerboseLvl) {
		args = append(args, "--verbose")
	}

	var serviceNames []string
	for _, s := range specs {
		serviceNames = append(serviceNames, s.Service)
	}

	args = append(args, []string{"rm", "--stop", "--force"}...)
	args = append(args, serviceNames...)
	cmd := c.dcCommand(ctx, args)
	cmd.Stdin = strings.NewReader(p.YAML)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil {
		return FormatError(cmd, nil, err)
	}

	return nil
}

func (c *cmdDCClient) StreamLogs(ctx context.Context, spec model.DockerComposeUpSpec) io.ReadCloser {
	args := c.projectArgs(spec.Project)

	r, w := io.Pipe()

	// NOTE: we can't practically remove "--no-color" due to the way that Docker Compose formats colorful lines; it
	// 		 will wrap the entire line (including the \n) with the color codes, so you end up with something like:
	//			\u001b[36mmyproject_my-container_1 exited with code 0\n\u001b[0m
	// 		 where the ANSI reset (\u001b[0m) is _AFTER_ the \n, which doesn't play nice with our log segment logic
	// 		 under some conditions - adding a final \n after stdout is closed would probably be sufficient given the
	// 		 current pattern of how Compose colorizes stuff, but it's really not worth the headache to find out
	args = append(args, "logs", "--no-color", "--no-log-prefix", "--timestamps", "--follow", spec.Service)
	cmd := c.dcCommand(ctx, args)
	cmd.Stdin = strings.NewReader(spec.Project.YAML)
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

func (c *cmdDCClient) StreamEvents(ctx context.Context, p model.DockerComposeProject) (<-chan string, error) {
	ch := make(chan string)

	args := c.projectArgs(p)
	args = append(args, "events", "--json")
	cmd := c.dcCommand(ctx, args)
	cmd.Stdin = strings.NewReader(p.YAML)
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

func (c *cmdDCClient) Project(ctx context.Context, spec model.DockerComposeProject) (*types.Project, error) {
	var proj *types.Project
	var err error

	// First, use compose-go to natively load the project.
	if len(spec.ConfigPaths) > 0 {
		parsed, err := c.loadProjectNative(spec.ConfigPaths)
		if err == nil {
			proj = parsed
		}
	}

	// HACK(milas): compose-go has known regressions with resolving variables during YAML loading
	// 	if it fails, attempt to fallback to using the CLI to resolve the YAML and then parse
	// 	it with compose-go
	// 	see https://github.com/tilt-dev/tilt/issues/4795
	if proj == nil {
		proj, err = c.loadProjectCLI(ctx, spec)
		if err != nil {
			return nil, err
		}
	}

	return proj, nil
}

func (c *cmdDCClient) ContainerID(ctx context.Context, spec model.DockerComposeUpSpec) (container.ID, error) {
	id, err := c.dcOutput(ctx, spec.Project, "ps", "-q", spec.Service)
	if err != nil {
		return container.ID(""), err
	}

	return container.ID(id), nil
}

// Version runs `docker-compose version` and parses the output, returning the canonical version and build (if present).
//
// NOTE: The version subcommand was added in Docker Compose v1.4.0 (released 2015-08-04), so this won't work for
// 		 truly ancient versions, but handles both v1 and v2.
func (c *cmdDCClient) Version(ctx context.Context) (string, string, error) {
	cmd := c.dcCommand(ctx, []string{"version"})
	stdout, err := cmd.Output()
	if err != nil {
		return "", "", FormatError(cmd, stdout, err)
	}
	return parseComposeVersionOutput(stdout)
}

func (c *cmdDCClient) loadProjectNative(configPaths []string) (*types.Project, error) {
	// NOTE: take care to keep behavior in sync with loadProjectCLI()
	opts, err := compose.NewProjectOptions(configPaths, dcProjectOptions...)
	if err != nil {
		return nil, err
	}
	proj, err := compose.ProjectFromOptions(opts)
	if err != nil {
		return nil, err
	}
	return proj, nil
}

func (c *cmdDCClient) loadProjectCLI(ctx context.Context, proj model.DockerComposeProject) (*types.Project, error) {
	resolvedYAML, err := c.dcOutput(ctx, proj, "config")
	if err != nil {
		return nil, err
	}

	// docker-compose is very inconsistent about whether it fully resolves paths or not via CLI, both between
	// v1 and v2 as well as even different releases within v2, so set the workdir and force the loader to resolve
	// any relative paths
	workDir := proj.ProjectPath
	if len(proj.ConfigPaths) != 0 {
		// from the compose Docs:
		// 	> When you use multiple Compose files, all paths in the files are relative to the first configuration file specified with -f
		// https://docs.docker.com/compose/reference/#use--f-to-specify-name-and-path-of-one-or-more-compose-files
		workDir = filepath.Dir(proj.ConfigPaths[0])
	}

	return loader.Load(types.ConfigDetails{
		WorkingDir: workDir,
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(resolvedYAML),
			},
		},
		// no environment specified because the CLI call will already have resolved all variables
	}, dcLoaderOption)
}

// dcLoaderOption is used when loading Docker Compose projects via the CLI and fallback and for tests.
//
// See also: dcProjectOptions which is used for loading projects from the Go library, which should
// be kept in sync behavior-wise.
func dcLoaderOption(opts *loader.Options) {
	opts.ResolvePaths = true
	opts.SkipNormalization = false
	opts.SkipInterpolation = false
}

func dcExecutablePath() string {
	v1Name := "docker-compose-v1"
	if runtime.GOOS == "windows" {
		v1Name += ".exe"
	}
	composePath, err := exec.LookPath(v1Name)
	if err != nil {
		composePath = "docker-compose"
	}
	return composePath
}

func (c *cmdDCClient) dcCommand(ctx context.Context, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.composePath, args...)
	cmd.Env = append(os.Environ(), c.env.AsEnviron()...)
	return cmd
}

func (c *cmdDCClient) dcOutput(ctx context.Context, p model.DockerComposeProject, args ...string) (string, error) {

	tempArgs := c.projectArgs(p)
	args = append(tempArgs, args...)
	cmd := c.dcCommand(ctx, args)
	cmd.Stdin = strings.NewReader(p.YAML)

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

// parseComposeVersionOutput parses the raw output of `docker-compose version` for both v1.x + v2.x Compose
// and returns the canonical semver + build (might be blank) or an error.
func parseComposeVersionOutput(stdout []byte) (string, string, error) {
	// match 0: raw output
	// match 1: version w/o leading v (required)
	// match 2: build (optional)
	m := versionRegex.FindSubmatch(bytes.TrimSpace(stdout))
	if len(m) < 2 {
		return "", "", fmt.Errorf("could not parse version from output: %q", string(stdout))
	}
	rawVersion := "v" + string(m[1])
	canonicalVersion := semver.Canonical(rawVersion)
	if canonicalVersion == "" {
		return "", "", fmt.Errorf("invalid version: %q", rawVersion)
	}
	build := semver.Build(rawVersion)
	if build != "" {
		// prefer semver build if present, but strip off the leading `+`
		// (currently, Docker Compose has not made use of this, preferring to list the build independently if at all)
		build = strings.TrimPrefix(build, "+")
	} else if len(m) > 2 {
		// otherwise, fall back to regex match if possible
		build = string(m[2])
	}
	return canonicalVersion, build, nil
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
