package docker

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/cli/opts"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/minikube"
)

// See notes on CreateClientOpts. These environment variables are standard docker env configs.
type Env struct {
	Host       string
	APIVersion string
	TLSVerify  string
	CertPath   string

	// Minikube's docker client has a bug where it can't use buildkit. See:
	// https://github.com/kubernetes/minikube/issues/4143
	IsMinikube bool

	// If the env failed to load for some reason, propagate that error
	// so that we can report it when the user tries to do a docker_build.
	Error error
}

// Serializes this back to environment variables for os.Environ
func (e Env) AsEnviron() []string {
	vars := []string{}
	if e.Host != "" {
		vars = append(vars, fmt.Sprintf("DOCKER_HOST=%s", e.Host))
	}
	if e.APIVersion != "" {
		vars = append(vars, fmt.Sprintf("DOCKER_API_VERSION=%s", e.APIVersion))
	}
	if e.CertPath != "" {
		vars = append(vars, fmt.Sprintf("DOCKER_CERT_PATH=%s", e.CertPath))
	}
	if e.TLSVerify != "" {
		vars = append(vars, fmt.Sprintf("DOCKER_TLS_VERIFY=%s", e.TLSVerify))
	}
	return vars
}

// Tell wire to create two docker envs: one for the local CLI and one for the in-cluster CLI.
type ClusterEnv Env
type LocalEnv Env

func ProvideLocalEnv(ctx context.Context, cEnv ClusterEnv) LocalEnv {
	result := overlayOSEnvVars(Env{})

	// The user may have already configured their local docker client
	// to use Minikube's docker server. We check for that by comparing
	// the hosts of the LocalEnv and ClusterEnv.
	if cEnv.Host == result.Host {
		result.IsMinikube = cEnv.IsMinikube
	}

	return LocalEnv(result)
}

func ProvideClusterEnv(ctx context.Context, env k8s.Env, runtime container.Runtime, minikubeClient minikube.Client) ClusterEnv {
	result := Env{}

	if runtime == container.RuntimeDocker {
		if env == k8s.EnvMinikube {
			// If we're running Minikube with a docker runtime, talk to Minikube's docker socket.
			envMap, err := minikubeClient.DockerEnv(ctx)
			if err != nil {
				return ClusterEnv{Error: err}
			}

			host := envMap["DOCKER_HOST"]
			if host != "" {
				result.Host = host
			}

			apiVersion := envMap["DOCKER_API_VERSION"]
			if apiVersion != "" {
				result.APIVersion = apiVersion
			}

			certPath := envMap["DOCKER_CERT_PATH"]
			if certPath != "" {
				result.CertPath = certPath
			}

			tlsVerify := envMap["DOCKER_TLS_VERIFY"]
			if tlsVerify != "" {
				result.TLSVerify = tlsVerify
			}

			result.IsMinikube = true
		} else if env == k8s.EnvMicroK8s {
			// If we're running Microk8s with a docker runtime, talk to Microk8s's docker socket.
			result.Host = microK8sDockerHost
		}
	}

	return ClusterEnv(overlayOSEnvVars(Env(result)))
}

func overlayOSEnvVars(result Env) Env {
	host := os.Getenv("DOCKER_HOST")
	if host != "" {
		host, err := opts.ParseHost(true, host)
		if err != nil {
			return Env{Error: errors.Wrap(err, "ProvideDockerEnv")}
		}

		// If the docker host is set from the env and different from the cluster host,
		// ignore all the variables from minikube/microk8s
		if host != result.Host {
			result = Env{Host: host}
		}
	}

	apiVersion := os.Getenv("DOCKER_API_VERSION")
	if apiVersion != "" {
		result.APIVersion = apiVersion
	}

	certPath := os.Getenv("DOCKER_CERT_PATH")
	if certPath != "" {
		result.CertPath = certPath
	}

	tlsVerify := os.Getenv("DOCKER_TLS_VERIFY")
	if tlsVerify != "" {
		result.TLSVerify = tlsVerify
	}

	return result
}
