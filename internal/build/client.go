package build

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/minikube"
)

// Use client for docker 17
// https://docs.docker.com/develop/sdk/#api-version-matrix
// API version 1.30 is the first version where the full digest
// shows up in the API output of BuildImage
const minDockerVersion = "1.30"

// Create an interface so this can be mocked out.
type DockerClient interface {
	ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error)
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImageTag(ctx context.Context, source, target string) error
}

func DefaultDockerClient(ctx context.Context, env k8s.Env) (*client.Client, error) {
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
	return d, nil
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
