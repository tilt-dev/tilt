package docker

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/distribution/reference"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types"
	typescontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	typesimage "github.com/docker/docker/api/types/image"
	typesregistry "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/registry"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/filesync"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker/buildkit"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const (
	// Indicates that an image was built by tilt's docker client.
	BuiltLabel = "dev.tilt.built"

	// Indicates that an image is eligible for garbage collection
	// by Tilt's pruner.
	GCEnabledLabel = "dev.tilt.gc"
)

var (
	BuiltLabelSet = map[string]string{
		BuiltLabel:     "true",
		GCEnabledLabel: "true",
	}
)

const clientSessionRemote = "client-session"

// Minimum docker version we've tested on.
// A good way to test old versions is to connect to an old version of Minikube,
// so that we connect to the docker server in minikube instead of futzing with
// the docker version on your machine.
// https://github.com/kubernetes/minikube/releases/tag/v0.13.1
var minDockerVersion = semver.MustParse("1.23.0")

var minDockerVersionStableBuildkit = semver.MustParse("1.39.0")
var minDockerVersionExperimentalBuildkit = semver.MustParse("1.38.0")

var versionTimeout = 5 * time.Second

// Create an interface so this can be mocked out.
type Client interface {
	CheckConnected() error

	// If you'd like to call this Docker instance in a separate process, these
	// are the environment variables you'll need to do so.
	Env() Env

	// If you'd like to call this Docker instance in a separate process, this
	// is the default builder version you want (buildkit or legacy)
	BuilderVersion(ctx context.Context) (types.BuilderVersion, error)

	ServerVersion(ctx context.Context) (types.Version, error)

	// Set the orchestrator we're talking to. This is only relevant to switchClient,
	// which can talk to either the Local or in-cluster docker daemon.
	SetOrchestrator(orc model.Orchestrator)
	// Return a client suitable for use with the given orchestrator. Only
	// relevant for the switchClient which has clients for both types.
	ForOrchestrator(orc model.Orchestrator) Client

	ContainerLogs(ctx context.Context, container string, options typescontainer.LogsOptions) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerList(ctx context.Context, options typescontainer.ListOptions) ([]types.Container, error)
	ContainerRestartNoWait(ctx context.Context, containerID string) error

	Run(ctx context.Context, opts RunConfig) (RunResult, error)

	// Execute a command in a container, streaming the command output to `out`.
	// Returns an ExitError if the command exits with a non-zero exit code.
	ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, in io.Reader, out io.Writer) error

	ImagePull(ctx context.Context, ref reference.Named) (reference.Canonical, error)
	ImagePush(ctx context.Context, image reference.NamedTagged) (io.ReadCloser, error)
	ImageBuild(ctx context.Context, g *errgroup.Group, buildContext io.Reader, options BuildOptions) (types.ImageBuildResponse, error)
	ImageTag(ctx context.Context, source, target string) error
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImageList(ctx context.Context, options typesimage.ListOptions) ([]typesimage.Summary, error)
	ImageRemove(ctx context.Context, imageID string, options typesimage.RemoveOptions) ([]typesimage.DeleteResponse, error)

	NewVersionError(ctx context.Context, APIrequired, feature string) error
	BuildCachePrune(ctx context.Context, opts types.BuildCachePruneOptions) (*types.BuildCachePruneReport, error)
	ContainersPrune(ctx context.Context, pruneFilters filters.Args) (types.ContainersPruneReport, error)
}

// Add-on interface for a client that manages multiple clients transparently.
type CompositeClient interface {
	Client
	DefaultLocalClient() Client
	DefaultClusterClient() Client
	ClientFor(cluster v1alpha1.Cluster) Client
	HasMultipleClients() bool
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

	authConfigsOnce func() map[string]typesregistry.AuthConfig
	env             Env

	versionsOnce   sync.Once
	builderVersion types.BuilderVersion
	serverVersion  types.Version
	versionError   error
}

func NewDockerClient(ctx context.Context, env Env) Client {
	if env.Error != nil {
		return newExplodingClient(env.Error)
	}

	d := env.Client.(*client.Client)

	return &Cli{
		Client:          d,
		env:             env,
		authConfigsOnce: sync.OnceValue(authConfigs),
	}
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
	if env.IsOldMinikube {
		// Buildkit for Minikube is busted on some versions. See
		// https://github.com/kubernetes/minikube/issues/4143
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
func CreateClientOpts(envMap map[string]string) ([]client.Opt, error) {
	result := make([]client.Opt, 0)

	certPath := envMap["DOCKER_CERT_PATH"]
	tlsVerify := envMap["DOCKER_TLS_VERIFY"]
	if certPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(certPath, "ca.pem"),
			CertFile:           filepath.Join(certPath, "cert.pem"),
			KeyFile:            filepath.Join(certPath, "key.pem"),
			InsecureSkipVerify: tlsVerify == "",
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

	host := envMap["DOCKER_HOST"]
	if host != "" {
		result = append(result, client.WithHost(host))
	}

	apiVersion := envMap["DOCKER_API_VERSION"]
	if apiVersion != "" {
		result = append(result, client.WithVersion(apiVersion))
	} else {
		// WithAPIVersionNegotiation makes the Docker client negotiate down to a lower
		// version if Docker's current API version is newer than the server version.
		result = append(result, client.WithAPIVersionNegotiation())
	}

	return result, nil
}

func (c *Cli) initVersion(ctx context.Context) {
	c.versionsOnce.Do(func() {
		ctx, cancel := context.WithTimeout(ctx, versionTimeout)
		defer cancel()

		serverVersion, err := c.Client.ServerVersion(ctx)
		if err != nil {
			c.versionError = err
			return
		}

		if !SupportedVersion(serverVersion) {
			c.versionError = fmt.Errorf("Tilt requires a Docker server newer than %s. Current Docker server: %s",
				minDockerVersion, serverVersion.APIVersion)
			return
		}

		builderVersion, err := getDockerBuilderVersion(serverVersion, c.env)
		if err != nil {
			c.versionError = err
			return
		}

		c.builderVersion = builderVersion
		c.serverVersion = serverVersion
	})
}

func (c *Cli) startBuildkitSession(ctx context.Context, g *errgroup.Group, key string, dirSource filesync.DirSource, sshSpecs []string, secretSpecs []string) (*session.Session, error) {
	session, err := session.NewSession(ctx, "tilt", key)
	if err != nil {
		return nil, err
	}

	if dirSource != nil {
		session.Allow(filesync.NewFSSyncProvider(dirSource))
	}

	dockerConfig := config.LoadDefaultConfigFile(
		logger.Get(ctx).Writer(logger.InfoLvl))
	provider := authprovider.NewDockerAuthProvider(dockerConfig, nil)
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

	g.Go(func() error {
		defer func() {
			_ = session.Close()
		}()

		// Start the server
		dialSession := func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
			return c.Client.DialHijack(ctx, "/session", proto, meta)
		}
		return session.Run(ctx, dialSession)
	})
	return session, nil
}

// When we pull from a private docker registry, we have to get credentials
// from somewhere. These credentials are not stored on the server. The client
// is responsible for managing them.
//
// Docker uses two different protocols:
//  1. In the legacy build engine, you have to get all the creds ahead of time
//     and pass them in the ImageBuild call.
//  2. In BuildKit, you have to create a persistent session. The client
//     side of the session manages a miniature server that just responds
//     to credential requests as the server asks for them.
//
// Protocol (1) is very slow. If you're using the gcloud credential store,
// fetching all the creds ahead of time can take ~3 seconds.
// Protocol (2) is more efficient, but also more complex to manage. We manage it lazily.
func authConfigs() map[string]typesregistry.AuthConfig {
	configFile := config.LoadDefaultConfigFile(io.Discard)

	// If we fail to get credentials for some reason, that's OK.
	// even the docker CLI ignores this:
	// https://github.com/docker/cli/blob/23446275646041f9b598d64c51be24d5d0e49376/cli/command/image/build.go#L386
	credentials, _ := configFile.GetAllCredentials()
	authConfigs := make(map[string]typesregistry.AuthConfig, len(credentials))
	for k, auth := range credentials {
		authConfigs[k] = typesregistry.AuthConfig(auth)
	}
	return authConfigs
}

func (c *Cli) CheckConnected() error                  { return nil }
func (c *Cli) SetOrchestrator(orc model.Orchestrator) {}
func (c *Cli) ForOrchestrator(orc model.Orchestrator) Client {
	return c
}
func (c *Cli) Env() Env {
	return c.env
}

func (c *Cli) BuilderVersion(ctx context.Context) (types.BuilderVersion, error) {
	c.initVersion(ctx)
	return c.builderVersion, c.versionError
}

func (c *Cli) ServerVersion(ctx context.Context) (types.Version, error) {
	c.initVersion(ctx)
	return c.serverVersion, c.versionError
}

type encodedAuth string

func (c *Cli) authInfo(ctx context.Context, repoInfo *registry.RepositoryInfo, cmdName string) (encodedAuth, types.RequestPrivilegeFunc, error) {
	cli, err := newDockerCli(ctx)
	if err != nil {
		return "", nil, errors.Wrap(err, "authInfo")
	}
	authConfig := command.ResolveAuthConfig(cli.ConfigFile(), repoInfo.Index)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(cli, repoInfo.Index, cmdName)

	auth, err := typesregistry.EncodeAuthConfig(authConfig)
	if err != nil {
		return "", nil, errors.Wrap(err, "authInfo#EncodeAuthConfig")
	}
	return encodedAuth(auth), requestPrivilege, nil
}

func (c *Cli) ImagePull(ctx context.Context, ref reference.Named) (reference.Canonical, error) {
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, fmt.Errorf("could not parse registry for %q: %v", ref.String(), err)
	}

	encodedAuth, requestPrivilege, err := c.authInfo(ctx, repoInfo, "push")
	if err != nil {
		return nil, fmt.Errorf("could not authenticate: %v", err)
	}

	image := ref.String()
	pullResp, err := c.Client.ImagePull(ctx, image, typesimage.PullOptions{
		RegistryAuth:  string(encodedAuth),
		PrivilegeFunc: requestPrivilege,
	})
	if err != nil {
		return nil, fmt.Errorf("could not pull image %q: %v", image, err)
	}
	defer func() {
		_ = pullResp.Close()
	}()

	// the /images/create API is a bit chaotic, returning JSON lines of status as it pulls
	// including ASCII progress bar animation etc.
	// there's not really any guarantees with it, so the prevailing guidance is to try and
	// inspect the image immediately afterwards to ensure it was pulled successfully
	// (this is racy and could be improved by _trying_ to get the digest out of this response
	// and making sure it matches with the result of inspect, but Docker itself suffers from
	// this same race condition during a docker run that triggers a pull, so it's reasonable
	// to deem it as acceptable here as well)
	_, err = io.Copy(io.Discard, pullResp)
	if err != nil {
		return nil, fmt.Errorf("connection error while pulling image %q: %v", image, err)
	}

	imgInspect, _, err := c.ImageInspectWithRaw(ctx, image)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect after pull for image %q: %v", image, err)
	}

	pulledRef, err := reference.ParseNormalizedNamed(imgInspect.RepoDigests[0])
	if err != nil {
		return nil, fmt.Errorf("invalid reference %q for image %q: %v", imgInspect.RepoDigests[0], image, err)
	}
	cRef, ok := pulledRef.(reference.Canonical)
	if !ok {
		// this indicates a bug/behavior change within Docker because we just parsed a digest reference
		return nil, fmt.Errorf("reference %q is not canonical", pulledRef.String())
	}
	// the reference from the repo digest will be missing the tag (if specified), so we attach the digest to the
	// original reference to get something like `docker.io/library/nginx:1.21.32@sha256:<hash>` for an input of
	// `docker.io/library/nginx:1.21.3` (if we used the repo digest, it'd be `docker.io/library/nginx@sha256:<hash>`
	// with no tag, so this ensures all parts are preserved).
	cRef, err = reference.WithDigest(ref, cRef.Digest())
	if err != nil {
		return nil, fmt.Errorf("invalid digest for reference %q: %v", pulledRef.String(), err)
	}
	return cRef, nil
}

func (c *Cli) ImagePush(ctx context.Context, ref reference.NamedTagged) (io.ReadCloser, error) {
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, errors.Wrap(err, "ImagePush#ParseRepositoryInfo")
	}

	logger.Get(ctx).Infof("Authenticating to image repo: %s", repoInfo.Index.Name)
	encodedAuth, requestPrivilege, err := c.authInfo(ctx, repoInfo, "push")
	if err != nil {
		return nil, errors.Wrap(err, "ImagePush: authenticate")
	}

	options := typesimage.PushOptions{
		RegistryAuth:  string(encodedAuth),
		PrivilegeFunc: requestPrivilege,
	}

	if reference.Domain(ref) == "" {
		return nil, errors.New("ImagePush: no domain in container name")
	}
	logger.Get(ctx).Infof("Sending image data")
	return c.Client.ImagePush(ctx, ref.String(), options)
}

func (c *Cli) ImageBuild(ctx context.Context, g *errgroup.Group, buildContext io.Reader, options BuildOptions) (types.ImageBuildResponse, error) {
	// Always use a one-time session when using buildkit, since credential
	// passing is fast and we want to get the latest creds.
	// https://github.com/tilt-dev/tilt/issues/4043
	var oneTimeSession *session.Session
	sessionID := ""

	mustUseBuildkit := len(options.SSHSpecs) > 0 || len(options.SecretSpecs) > 0 || options.DirSource != nil
	builderVersion, err := c.BuilderVersion(ctx)
	if err != nil {
		return types.ImageBuildResponse{}, err
	}
	if options.ForceLegacyBuilder {
		builderVersion = types.BuilderV1
	}

	isUsingBuildkit := builderVersion == types.BuilderBuildKit
	if isUsingBuildkit {
		var err error
		oneTimeSession, err = c.startBuildkitSession(ctx, g, identity.NewID(), options.DirSource, options.SSHSpecs, options.SecretSpecs)
		if err != nil {
			return types.ImageBuildResponse{}, errors.Wrapf(err, "ImageBuild")
		}
		sessionID = oneTimeSession.ID()
	} else if mustUseBuildkit {
		return types.ImageBuildResponse{},
			fmt.Errorf("Docker SSH secrets only work on Buildkit, but Buildkit has been disabled")
	}

	opts := types.ImageBuildOptions{}
	opts.Version = builderVersion

	if isUsingBuildkit {
		opts.SessionID = sessionID
	} else {
		opts.AuthConfigs = c.authConfigsOnce()
	}

	opts.Remove = options.Remove
	opts.Context = options.Context
	opts.BuildArgs = options.BuildArgs
	opts.Dockerfile = options.Dockerfile
	opts.Tags = append([]string{}, options.ExtraTags...)
	opts.Target = options.Target
	opts.NetworkMode = options.Network
	opts.CacheFrom = options.CacheFrom
	opts.PullParent = options.PullParent
	opts.Platform = options.Platform
	opts.ExtraHosts = append([]string{}, options.ExtraHosts...)

	if options.DirSource != nil {
		opts.RemoteContext = clientSessionRemote
	}

	opts.Labels = BuiltLabelSet // label all images as built by us

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

func (c *Cli) ContainerRestartNoWait(ctx context.Context, containerID string) error {

	// Don't wait on the container to fully start.
	dur := 0

	return c.ContainerRestart(ctx, containerID, typescontainer.StopOptions{
		Timeout: &dur,
	})
}

func (c *Cli) ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, in io.Reader, out io.Writer) error {
	attachStdin := in != nil
	cfg := types.ExecConfig{
		Cmd:          cmd.Argv,
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  attachStdin,
		Tty:          !attachStdin,
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

	err = c.ContainerExecStart(ctx, execId.ID, types.ExecStartCheck{})
	if err != nil {
		return errors.Wrap(err, "ExecInContainer#start")
	}

	_, err = fmt.Fprintf(out, "RUNNING: %s\n", cmd)
	if err != nil {
		return errors.Wrap(err, "ExecInContainer#print")
	}

	inputDone := make(chan struct{})
	if attachStdin {
		go func() {
			_, err := io.Copy(connection.Conn, in)
			if err != nil {
				logger.Get(ctx).Debugf("copy error: %v", err)
			}
			err = connection.CloseWrite()
			if err != nil {
				logger.Get(ctx).Debugf("close write error: %v", err)
			}
			close(inputDone)
		}()
	} else {
		close(inputDone)
	}

	_, err = io.Copy(out, connection.Reader)
	if err != nil {
		return errors.Wrap(err, "ExecInContainer#copy")
	}

	<-inputDone

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

func (c *Cli) Run(ctx context.Context, opts RunConfig) (RunResult, error) {
	if opts.Pull {
		namedRef, ok := opts.Image.(reference.Named)
		if !ok {
			return RunResult{}, fmt.Errorf("invalid reference type %T for pull", opts.Image)
		}
		if _, err := c.ImagePull(ctx, namedRef); err != nil {
			return RunResult{}, fmt.Errorf("error pulling image %q: %v", opts.Image, err)
		}
	}

	cc := &typescontainer.Config{
		Image:        opts.Image.String(),
		AttachStdout: opts.Stdout != nil,
		AttachStderr: opts.Stderr != nil,
		Cmd:          opts.Cmd,
		Labels:       BuiltLabelSet,
	}

	hc := &typescontainer.HostConfig{
		Mounts: opts.Mounts,
	}

	createResp, err := c.Client.ContainerCreate(ctx,
		cc,
		hc,
		nil,
		nil,
		opts.ContainerName,
	)
	if err != nil {
		return RunResult{}, fmt.Errorf("could not create container: %v", err)
	}

	tearDown := func(containerID string) error {
		return c.Client.ContainerRemove(ctx, createResp.ID, typescontainer.RemoveOptions{Force: true})
	}

	var containerStarted bool
	defer func(containerID string) {
		// make an effort to clean up any container we create but don't successfully start
		if containerStarted {
			return
		}
		if err := tearDown(containerID); err != nil {
			logger.Get(ctx).Debugf("Failed to remove container after error before start (id=%s): %v", createResp.ID, err)
		}
	}(createResp.ID)

	statusCh, statusErrCh := c.Client.ContainerWait(ctx, createResp.ID, typescontainer.WaitConditionNextExit)
	// ContainerWait() can immediately write to the error channel before returning if it can't start the API request,
	// so catch these errors early (it _also_ can write to that channel later, so it's still passed to the RunResult)
	select {
	case err = <-statusErrCh:
		return RunResult{}, fmt.Errorf("could not wait for container (id=%s): %v", createResp.ID, err)
	default:
	}

	err = c.Client.ContainerStart(ctx, createResp.ID, typescontainer.StartOptions{})
	if err != nil {
		return RunResult{}, fmt.Errorf("could not start container (id=%s): %v", createResp.ID, err)
	}
	containerStarted = true

	logsErrCh := make(chan error, 1)
	if opts.Stdout != nil || opts.Stderr != nil {
		var logsResp io.ReadCloser
		logsResp, err = c.Client.ContainerLogs(
			ctx, createResp.ID, typescontainer.LogsOptions{
				ShowStdout: opts.Stdout != nil,
				ShowStderr: opts.Stderr != nil,
				Follow:     true,
			},
		)
		if err != nil {
			return RunResult{}, fmt.Errorf("could not read container logs: %v", err)
		}

		go func() {
			stdout := opts.Stdout
			if stdout == nil {
				stdout = io.Discard
			}
			stderr := opts.Stderr
			if stderr == nil {
				stderr = io.Discard
			}

			_, err = stdcopy.StdCopy(stdout, stderr, logsResp)
			_ = logsResp.Close()
			logsErrCh <- err
		}()
	} else {
		// there is no I/O so immediately signal so that the result call doesn't block on it
		logsErrCh <- nil
	}

	result := RunResult{
		ContainerID:  createResp.ID,
		logsErrCh:    logsErrCh,
		statusRespCh: statusCh,
		statusErrCh:  statusErrCh,
		tearDown:     tearDown,
	}

	return result, nil
}
