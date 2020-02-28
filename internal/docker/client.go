package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/config"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/registry"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker/buildkit"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

// Label that we attach to all of the images we build.
const (
	BuiltByLabel = "builtby"
	BuiltByValue = "tilt"
)

var (
	BuiltByTiltLabel    = map[string]string{BuiltByLabel: BuiltByValue}
	BuiltByTiltLabelStr = fmt.Sprintf("%s=%s", BuiltByLabel, BuiltByValue)
)

// Version info
// https://docs.docker.com/develop/sdk/#api-version-matrix
//
// The docker API docs highly recommend we set a default version,
// so that new versions don't break us.
const defaultVersion = "1.39"

// Minimum docker version we've tested on.
// A good way to test old versions is to connect to an old version of Minikube,
// so that we connect to the docker server in minikube instead of futzing with
// the docker version on your machine.
// https://github.com/kubernetes/minikube/releases/tag/v0.13.1
var minDockerVersion = semver.MustParse("1.23.0")

var minDockerVersionStableBuildkit = semver.MustParse("1.39.0")
var minDockerVersionExperimentalBuildkit = semver.MustParse("1.38.0")

// microk8s exposes its own docker socket
// https://github.com/ubuntu/microk8s/blob/master/docs/dockerd.md
const microK8sDockerHost = "unix:///var/snap/microk8s/current/docker.sock"

// generate a session key
var sessionSharedKey = identity.NewID()

// Create an interface so this can be mocked out.
type Client interface {
	CheckConnected() error

	// If you'd like to call this Docker instance in a separate process, these
	// are the environment variables you'll need to do so.
	Env() Env

	// If you'd like to call this Docker instance in a separate process, this
	// is the default builder version you want (buildkit or legacy)
	BuilderVersion() types.BuilderVersion

	ServerVersion() types.Version

	// Set the orchestrator we're talking to. This is only relevant to switchClient,
	// which can talk to either the Local or in-cluster docker daemon.
	SetOrchestrator(orc model.Orchestrator)

	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerRestartNoWait(ctx context.Context, containerID string) error
	CopyToContainerRoot(ctx context.Context, container string, content io.Reader) error

	// Execute a command in a container, streaming the command output to `out`.
	// Returns an ExitError if the command exits with a non-zero exit code.
	ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, out io.Writer) error

	ImagePush(ctx context.Context, image reference.NamedTagged) (io.ReadCloser, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options BuildOptions) (types.ImageBuildResponse, error)
	ImageTag(ctx context.Context, source, target string) error
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error)
	ImageRemove(ctx context.Context, imageID string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)

	NewVersionError(APIrequired, feature string) error
	BuildCachePrune(ctx context.Context, opts types.BuildCachePruneOptions) (*types.BuildCachePruneReport, error)
	ContainersPrune(ctx context.Context, pruneFilters filters.Args) (types.ContainersPruneReport, error)
}

type ExitError struct {
	ExitCode int
}

func (e ExitError) Error() string {
	return fmt.Sprintf("Exec command exited with status code: %d", e.ExitCode)
}

func IsExitError(err error) bool {
	_, ok := err.(ExitError)
	return ok
}

var _ error = ExitError{}

var _ Client = &Cli{}

type Cli struct {
	*client.Client
	builderVersion types.BuilderVersion
	serverVersion  types.Version

	creds     dockerCreds
	initError error
	initDone  chan bool
	env       Env
}

func NewDockerClient(ctx context.Context, env Env) Client {
	if env.Error != nil {
		return newExplodingClient(env.Error)
	}

	opts, err := CreateClientOpts(ctx, env)
	if err != nil {
		return newExplodingClient(err)
	}
	d, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return newExplodingClient(err)
	}

	serverVersion, err := d.ServerVersion(ctx)
	if err != nil {
		return newExplodingClient(err)
	}

	if !SupportedVersion(serverVersion) {
		return newExplodingClient(
			fmt.Errorf("Tilt requires a Docker server newer than %s. Current Docker server: %s",
				minDockerVersion, serverVersion.APIVersion))
	}

	builderVersion, err := getDockerBuilderVersion(serverVersion, env)
	if err != nil {
		return newExplodingClient(err)
	}

	cli := &Cli{
		Client:         d,
		env:            env,
		builderVersion: builderVersion,
		serverVersion:  serverVersion,
		initDone:       make(chan bool),
	}

	go cli.backgroundInit(ctx)

	return cli
}

func SupportedVersion(v types.Version) bool {
	version, err := semver.ParseTolerant(v.APIVersion)
	if err != nil {
		// If the server version doesn't parse, we shouldn't even start
		return false
	}

	return version.GTE(minDockerVersion)
}

func getDockerBuilderVersion(v types.Version, env Env) (types.BuilderVersion, error) {
	// If the user has explicitly chosen to enable/disable buildkit, respect that.
	buildkitEnv := os.Getenv("DOCKER_BUILDKIT")
	if buildkitEnv != "" {
		buildkitEnabled, err := strconv.ParseBool(buildkitEnv)
		if err != nil {
			// This error message is copied from Docker, for consistency.
			return "", errors.Wrap(err, "DOCKER_BUILDKIT environment variable expects boolean value")
		}
		if buildkitEnabled && SupportsBuildkit(v, env) {
			return types.BuilderBuildKit, nil

		}
		return types.BuilderV1, nil
	}

	if SupportsBuildkit(v, env) {
		return types.BuilderBuildKit, nil
	}
	return types.BuilderV1, nil
}

// Sadly, certain versions of docker return an error if the client requests
// buildkit. We have to infer whether it supports buildkit from version numbers.
//
// Inferred from release notes
// https://docs.docker.com/engine/release-notes/
func SupportsBuildkit(v types.Version, env Env) bool {
	if env.IsMinikube {
		// Buildkit for Minikube is currently busted. Follow
		// https://github.com/kubernetes/minikube/issues/4143
		// for updates.
		return false
	}

	version, err := semver.ParseTolerant(v.APIVersion)
	if err != nil {
		// If the server version doesn't parse, disable buildkit
		return false
	}

	if minDockerVersionStableBuildkit.LTE(version) {
		return true
	}

	if minDockerVersionExperimentalBuildkit.LTE(version) && v.Experimental {
		return true
	}

	return false
}

// Adapted from client.FromEnv
//
// Supported environment variables:
// DOCKER_HOST to set the url to the docker server.
// DOCKER_API_VERSION to set the version of the API to reach, leave empty for latest.
// DOCKER_CERT_PATH to load the TLS certificates from.
// DOCKER_TLS_VERIFY to enable or disable TLS verification, off by default.
func CreateClientOpts(ctx context.Context, env Env) ([]client.Opt, error) {
	result := make([]client.Opt, 0)

	if env.CertPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(env.CertPath, "ca.pem"),
			CertFile:           filepath.Join(env.CertPath, "cert.pem"),
			KeyFile:            filepath.Join(env.CertPath, "key.pem"),
			InsecureSkipVerify: env.TLSVerify == "",
		}
		tlsc, err := tlsconfig.Client(options)
		if err != nil {
			return nil, err
		}

		result = append(result, client.WithHTTPClient(&http.Client{
			Transport:     &http.Transport{TLSClientConfig: tlsc},
			CheckRedirect: client.CheckRedirect,
		}))
	}

	if env.Host != "" {
		result = append(result, client.WithHost(env.Host))
	}

	if env.APIVersion != "" {
		result = append(result, client.WithVersion(env.APIVersion))
	} else {
		// WithAPIVersionNegotiation makes the Docker client negotiate down to a lower
		// version if Docker's current API version is newer than the server version.
		result = append(result, client.WithAPIVersionNegotiation())
	}

	return result, nil
}

type dockerCreds struct {
	authConfigs map[string]types.AuthConfig
	sessionID   string
}

func (c *Cli) startBuildkitSession(ctx context.Context, key string, sshSpecs []string, secretSpecs []string) (*session.Session, error) {
	session, err := session.NewSession(ctx, "tilt", key)
	if err != nil {
		return nil, err

	}

	provider := authprovider.NewDockerAuthProvider(logger.Get(ctx).Writer(logger.InfoLvl))
	session.Allow(provider)

	if len(secretSpecs) > 0 {
		ss, err := buildkit.ParseSecretSpecs(secretSpecs)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse secret: %v", secretSpecs)
		}
		session.Allow(ss)
	}

	if len(sshSpecs) > 0 {
		sshp, err := buildkit.ParseSSHSpecs(sshSpecs)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse ssh: %v", sshSpecs)
		}
		session.Allow(sshp)
	}

	go func() {
		defer func() {
			_ = session.Close()
		}()

		// Start the server
		dialSession := func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
			return c.Client.DialHijack(ctx, "/session", proto, meta)
		}
		_ = session.Run(ctx, dialSession)
	}()
	return session, nil
}

// When we pull from a private docker registry, we have to get credentials
// from somewhere. These credentials are not stored on the server. The client
// is responsible for managing them.
//
// Docker uses two different protocols:
// 1) In the legacy build engine, you have to get all the creds ahead of time
//    and pass them in the ImageBuild call.
// 2) In BuildKit, you have to create a persistent session. The client
//    side of the session manages a miniature server that just responds
//    to credential requests as the server asks for them.
//
// Protocol (1) is very slow. If you're using the gcloud credential store,
// fetching all the creds ahead of time can take ~3 seconds.
// Protocol (2) is more efficient, but also more complex to manage.
func (c *Cli) initCreds(ctx context.Context) dockerCreds {
	creds := dockerCreds{}

	if c.builderVersion == types.BuilderBuildKit {
		session, err := c.startBuildkitSession(ctx, sessionSharedKey, nil, nil)
		if err != nil {
			logger.Get(ctx).Warnf("Docker BuildKit session failed to init: %v", err)
		} else if session != nil {
			creds.sessionID = session.ID()
		}
	} else {
		configFile := config.LoadDefaultConfigFile(ioutil.Discard)

		// If we fail to get credentials for some reason, that's OK.
		// even the docker CLI ignores this:
		// https://github.com/docker/cli/blob/23446275646041f9b598d64c51be24d5d0e49376/cli/command/image/build.go#L386
		credentials, _ := configFile.GetAllCredentials()
		authConfigs := make(map[string]types.AuthConfig, len(credentials))
		for k, auth := range credentials {
			authConfigs[k] = types.AuthConfig(auth)
		}
		creds.authConfigs = authConfigs
	}

	return creds
}

// Initialization that we do in the background, because
// it may need to read from files or call out to gcloud.
//
// TODO(nick): Update ImagePush to use these auth credentials. This is less important
// for local k8s (Minikube, Docker-for-Mac, MicroK8s) because they don't push.
func (c *Cli) backgroundInit(ctx context.Context) {
	result := make(chan dockerCreds, 1)

	go func() {
		result <- c.initCreds(ctx)
	}()

	select {
	case creds := <-result:
		c.creds = creds
	case <-time.After(10 * time.Second):
		// TODO(nick): If we move logging before the wire() call, we should
		// print here instead of logging indirectly
		c.initError = fmt.Errorf("Timeout fetching docker auth credentials")
	}

	close(c.initDone)
}

func (c *Cli) CheckConnected() error                  { return nil }
func (c *Cli) SetOrchestrator(orc model.Orchestrator) {}
func (c *Cli) Env() Env {
	return c.env
}

func (c *Cli) BuilderVersion() types.BuilderVersion {
	return c.builderVersion
}

func (c *Cli) ServerVersion() types.Version {
	return c.serverVersion
}

func (c *Cli) ImagePush(ctx context.Context, ref reference.NamedTagged) (io.ReadCloser, error) {
	<-c.initDone

	if c.initError != nil {
		logger.Get(ctx).Verbosef("%v", c.initError)
	}

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, errors.Wrap(err, "ImagePush#ParseRepositoryInfo")
	}

	logger.Get(ctx).Infof("Authenticating to image repo: %s", repoInfo.Index.Name)
	infoWriter := logger.Get(ctx).Writer(logger.InfoLvl)
	cli, err := command.NewDockerCli(
		command.WithCombinedStreams(infoWriter),
		command.WithContentTrust(true),
	)
	if err != nil {
		return nil, errors.Wrap(err, "ImagePush#NewDockerCli")
	}

	err = cli.Initialize(cliflags.NewClientOptions())
	if err != nil {
		return nil, errors.Wrap(err, "ImagePush#InitializeCLI")
	}
	authConfig := command.ResolveAuthConfig(ctx, cli, repoInfo.Index)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(cli, repoInfo.Index, "push")

	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return nil, errors.Wrap(err, "ImagePush#EncodeAuthToBase64")
	}

	options := types.ImagePushOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	}

	if reference.Domain(ref) == "" {
		return nil, errors.Wrap(err, "ImagePush: no domain in container name")
	}
	logger.Get(ctx).Infof("Sending image data")
	return c.Client.ImagePush(ctx, ref.String(), options)
}

func (c *Cli) ImageBuild(ctx context.Context, buildContext io.Reader, options BuildOptions) (types.ImageBuildResponse, error) {
	<-c.initDone

	if c.initError != nil {
		logger.Get(ctx).Verbosef("%v", c.initError)
	}

	var oneTimeSession *session.Session
	sessionID := c.creds.sessionID
	if len(options.SSHSpecs) > 0 || len(options.SecretSpecs) > 0 {
		if c.builderVersion != types.BuilderBuildKit {
			return types.ImageBuildResponse{},
				fmt.Errorf("Docker SSH secrets only work on Buildkit, but Buildkit has been disabled")
		}

		var err error
		oneTimeSession, err = c.startBuildkitSession(ctx, identity.NewID(), options.SSHSpecs, options.SecretSpecs)
		if err != nil {
			return types.ImageBuildResponse{}, errors.Wrapf(err, "ImageBuild")
		}
		sessionID = oneTimeSession.ID()
	}

	opts := types.ImageBuildOptions{}
	opts.Version = c.builderVersion
	opts.AuthConfigs = c.creds.authConfigs
	opts.SessionID = sessionID
	opts.Remove = options.Remove
	opts.Context = options.Context
	opts.BuildArgs = options.BuildArgs
	opts.Dockerfile = options.Dockerfile
	opts.Tags = append([]string{}, options.ExtraTags...)
	opts.Target = options.Target
	opts.NetworkMode = options.Network

	opts.Labels = BuiltByTiltLabel // label all images as built by us

	response, err := c.Client.ImageBuild(ctx, buildContext, opts)
	if err != nil {
		if oneTimeSession != nil {
			_ = oneTimeSession.Close()
		}
		return response, err
	}

	if oneTimeSession != nil {
		response.Body = WrapReadCloserWithTearDown(response.Body, oneTimeSession.Close)
	}
	return response, err
}

func (c *Cli) CopyToContainerRoot(ctx context.Context, container string, content io.Reader) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-CopyToContainerRoot")
	defer span.Finish()
	return c.CopyToContainer(ctx, container, "/", content, types.CopyToContainerOptions{})
}

func (c *Cli) ContainerRestartNoWait(ctx context.Context, containerID string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ContainerRestartNoWait")
	defer span.Finish()

	// Don't wait on the container to fully start.
	dur := time.Duration(0)

	return c.ContainerRestart(ctx, containerID, &dur)
}

func (c *Cli) ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, out io.Writer) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "dockerCli-ExecInContainer")
	span.SetTag("cmd", strings.Join(cmd.Argv, " "))
	defer span.Finish()

	cfg := types.ExecConfig{
		Cmd:          cmd.Argv,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	// ContainerExecCreate error-handling is awful, so before we Create
	// we do a dummy inspect, to get more reasonable error messages. See:
	// https://github.com/docker/cli/blob/ae1618713f83e7da07317d579d0675f578de22fa/cli/command/container/exec.go#L77
	if _, err := c.ContainerInspect(ctx, cID.String()); err != nil {
		return errors.Wrap(err, "ExecInContainer")
	}

	execId, err := c.ContainerExecCreate(ctx, cID.String(), cfg)
	if err != nil {
		return errors.Wrap(err, "ExecInContainer#create")
	}

	connection, err := c.ContainerExecAttach(ctx, execId.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return errors.Wrap(err, "ExecInContainer#attach")
	}
	defer connection.Close()

	esSpan, ctx := opentracing.StartSpanFromContext(ctx, "dockerCli-ExecInContainer-ExecStart")
	err = c.ContainerExecStart(ctx, execId.ID, types.ExecStartCheck{})
	esSpan.Finish()
	if err != nil {
		return errors.Wrap(err, "ExecInContainer#start")
	}

	_, err = fmt.Fprintf(out, "RUNNING: %s\n", cmd)
	if err != nil {
		return errors.Wrap(err, "ExecInContainer#print")
	}

	bufSpan, ctx := opentracing.StartSpanFromContext(ctx, "dockerCli-ExecInContainer-readOutput")
	_, err = io.Copy(out, connection.Reader)
	bufSpan.Finish()
	if err != nil {
		return errors.Wrap(err, "ExecInContainer#copy")
	}

	for {
		inspected, err := c.ContainerExecInspect(ctx, execId.ID)
		if err != nil {
			return errors.Wrap(err, "ExecInContainer#inspect")
		}

		if inspected.Running {
			continue
		}

		status := inspected.ExitCode
		if status != 0 {
			return ExitError{ExitCode: status}
		}
		return nil
	}
}
