package build

import (
	"bytes"
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
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/minikube"
	"github.com/windmilleng/tilt/internal/model"
)

// Use client for docker 17
// https://docs.docker.com/develop/sdk/#api-version-matrix
// API version 1.30 is the first version where the full digest
// shows up in the API output of BuildImage
const minDockerVersion = "1.30"

// Create an interface so this can be mocked out.
type DockerClient interface {
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerRestartNoWait(ctx context.Context, containerID string) error
	CopyToContainerRoot(ctx context.Context, container string, content io.Reader) error
	ExecInContainer(ctx context.Context, cID k8s.ContainerID, cmd model.Cmd) error
	ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImageTag(ctx context.Context, source, target string) error
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error)
	ImageRemove(ctx context.Context, imageID string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)
}

var _ DockerClient = &DockerCli{}

type DockerCli struct {
	*client.Client
}

func DefaultDockerClient(ctx context.Context, env k8s.Env) (*DockerCli, error) {
	envFunc := os.Getenv
	if env == k8s.EnvMinikube {
		envMap, err := minikube.DockerEnv(ctx)
		if err != nil {
			return nil, fmt.Errorf("newDockerClient: %v", err)
		}

		envFunc = func(key string) string { return envMap[key] }
	}

	opts, err := CreateClientOpts(envFunc)
	if err != nil {
		return nil, fmt.Errorf("newDockerClient: %v", err)
	}
	d, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("newDockerClient: %v", err)
	}
	return &DockerCli{d}, nil
}

// Adapted from client.FromEnv
//
// Supported environment variables:
// DOCKER_HOST to set the url to the docker server.
// DOCKER_API_VERSION to set the version of the API to reach, leave empty for latest.
// DOCKER_CERT_PATH to load the TLS certificates from.
// DOCKER_TLS_VERIFY to enable or disable TLS verification, off by default.
func CreateClientOpts(env func(string) string) ([]func(client *client.Client) error, error) {
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

	minVersion, err := semver.ParseTolerant(minDockerVersion)
	if err != nil {
		return nil, fmt.Errorf("Minimum docker version is invalid: %s", minDockerVersion)
	}

	versionToSet := minVersion
	if version := env("DOCKER_API_VERSION"); version != "" {
		reqVersion, err := semver.ParseTolerant(version)
		if err != nil {
			return nil, fmt.Errorf("Could not parse DOCKER_API_VERSION: %s", version)
		}

		if minVersion.LT(reqVersion) {
			versionToSet = reqVersion
		}
	}

	result = append(result, client.WithVersion(versionToSet.String()))

	return result, nil
}

func (d *DockerCli) CopyToContainerRoot(ctx context.Context, container string, content io.Reader) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-CopyToContainerRoot")
	defer span.Finish()
	return d.CopyToContainer(ctx, container, "/", content, types.CopyToContainerOptions{})
}

func (d *DockerCli) ContainerRestartNoWait(ctx context.Context, containerID string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ContainerRestartNoWait")
	defer span.Finish()

	// Don't wait on the container to fully start.
	dur := time.Duration(0)

	return d.ContainerRestart(ctx, containerID, &dur)
}

func (d *DockerCli) ExecInContainer(ctx context.Context, cID k8s.ContainerID, cmd model.Cmd) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ExecInContainer")
	span.SetTag("cmd", strings.Join(cmd.Argv, " "))
	defer span.Finish()

	cfg := types.ExecConfig{
		Cmd:          cmd.Argv,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	execId, err := d.ContainerExecCreate(ctx, cID.String(), cfg)
	if err != nil {
		return fmt.Errorf("ExecInContainer#create: %v", err)
	}

	hijack, err := d.ContainerExecAttach(ctx, execId.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return fmt.Errorf("ExecInContainer#attach: %v", err)
	}
	defer hijack.Close()

	esSpan, ctx := opentracing.StartSpanFromContext(ctx, "dockerCli-ExecInContainer-ExecStart")
	err = d.ContainerExecStart(ctx, execId.ID, types.ExecStartCheck{})
	esSpan.Finish()
	if err != nil {
		return fmt.Errorf("ExecInContainer#start: %v", err)
	}

	bufSpan, ctx := opentracing.StartSpanFromContext(ctx, "dockerCli-ExecInContainer-readOutput")
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, hijack.Reader)
	bufSpan.Finish()
	if err != nil {
		return fmt.Errorf("ExecInContainer#copy: %v", err)
	}

	for true {
		inspected, err := d.ContainerExecInspect(ctx, execId.ID)
		if err != nil {
			return fmt.Errorf("ExecInContainer#inspect: %v", err)
		}

		if inspected.Running {
			continue
		}

		status := inspected.ExitCode
		if status != 0 {
			return fmt.Errorf("Failed with exit code %d. Output:\n%s", status, buf.String())
		}
		return nil
	}

	return nil
}
