package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/minikube"
	"github.com/windmilleng/tilt/internal/model"
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

// Create an interface so this can be mocked out.
type Client interface {
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerRestartNoWait(ctx context.Context, containerID string) error
	CopyToContainerRoot(ctx context.Context, container string, content io.Reader) error

	// Execute a command in a container, streaming the command output to `out`.
	// Returns an ExitError if the command exits with a non-zero exit code.
	ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, out io.Writer) error

	ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImageTag(ctx context.Context, source, target string) error
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error)
	ImageRemove(ctx context.Context, imageID string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)
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
	supportsBuildkit bool
}

func DefaultClient(ctx context.Context, env k8s.Env) (*Cli, error) {
	envFunc := os.Getenv
	if env == k8s.EnvMinikube {
		envMap, err := minikube.DockerEnv(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "defaultDockerClient")
		}

		envFunc = func(key string) string { return envMap[key] }
	} else if env == k8s.EnvMicroK8s {
		envFunc = func(key string) string {
			val := os.Getenv(key)
			if val == "" && key == "DOCKER_HOST" {
				return microK8sDockerHost
			}
			return val
		}
	}

	opts, err := CreateClientOpts(ctx, envFunc)
	if err != nil {
		return nil, errors.Wrap(err, "defaultDockerClient")
	}
	d, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, errors.Wrap(err, "defaultDockerClient")
	}

	serverVersion, err := d.ServerVersion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "defaultDockerClient")
	}

	if !SupportedVersion(serverVersion) {
		return nil, fmt.Errorf("Tilt requires a Docker server newer than %s. Current Docker server: %s",
			minDockerVersion, serverVersion.APIVersion)
	}

	return &Cli{
		Client:           d,
		supportsBuildkit: SupportsBuildkit(serverVersion),
	}, nil
}

func SupportedVersion(v types.Version) bool {
	version, err := semver.ParseTolerant(v.APIVersion)
	if err != nil {
		// If the server version doesn't parse, we shouldn't even start
		return false
	}

	return version.GTE(minDockerVersion)
}

// Sadly, certain versions of docker return an error if the client requests
// buildkit. We have to infer whether it supports buildkit from version numbers.
//
// Inferred from release notes
// https://docs.docker.com/engine/release-notes/
func SupportsBuildkit(v types.Version) bool {
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
func CreateClientOpts(ctx context.Context, env func(string) string) ([]func(client *client.Client) error, error) {
	result := make([]func(client *client.Client) error, 0)

	if dockerCertPath := env("DOCKER_CERT_PATH"); dockerCertPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
			CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
			KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
			InsecureSkipVerify: env("DOCKER_TLS_VERIFY") == "",
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

	if host := env("DOCKER_HOST"); host != "" {
		result = append(result, client.WithHost(host))
	}

	if version := env("DOCKER_API_VERSION"); version != "" {
		result = append(result, client.WithVersion(version))
	} else {
		// NegotateAPIVersion makes the docker client negotiate down to a lower version
		// if 'defaultVersion' is newer than the server version.
		result = append(result, client.WithVersion(defaultVersion), NegotiateAPIVersion(ctx))
	}

	return result, nil
}

func NegotiateAPIVersion(ctx context.Context) func(client *client.Client) error {
	return func(client *client.Client) error {
		client.NegotiateAPIVersion(ctx)
		return nil
	}
}

func (c *Cli) ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	requestedBuildkit := options.Version == types.BuilderBuildKit
	if requestedBuildkit && !c.supportsBuildkit {
		options.Version = types.BuilderV1
	}
	return c.Client.ImageBuild(ctx, buildContext, options)
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

	for true {
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

	return nil
}
