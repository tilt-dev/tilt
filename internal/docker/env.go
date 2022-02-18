package docker

import (
	"context"
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/docker/cli/opts"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// We didn't pick minikube v1.8.0 for any particular reason, it's just what Nick
// had on his machine and could verify with. It's hard to tell from the upstream
// issue when this was fixed.
//
// We can move it earlier if someone asks for it, though minikube is pretty good
// about nudging people to upgrade.
var minMinikubeVersionBuildkit = semver.MustParse("1.8.0")

// See notes on CreateClientOpts. These environment variables are standard docker env configs.
type Env struct {
	Host       string
	APIVersion string
	TLSVerify  string
	CertPath   string

	// Minikube's docker client has a bug where it can't use buildkit. See:
	// https://github.com/kubernetes/minikube/issues/4143
	IsOldMinikube bool

	// Some Kubernetes contexts have a Docker daemon that they use directly
	// as their container runtime. Any images built on that daemon will
	// show up automatically in the runtime.
	//
	// We used to store this as a property of the k8s env but now store it as a field of the Docker Env, because this
	// really affects how we interact with the Docker Env (rather than
	// how we interact with the K8s Env).
	//
	// In theory, you can have more than one, but in practice,
	// this is very difficult to set up.
	BuildToKubeContexts []string

	// If the env failed to load for some reason, propagate that error
	// so that we can report it when the user tries to do a docker_build.
	Error error
}

// Determines if this docker client can build images directly to the given cluster.
func (e Env) WillBuildToKubeContext(kctx k8s.KubeContext) bool {
	for _, current := range e.BuildToKubeContexts {
		if string(kctx) == current {
			return true
		}
	}
	return false
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

func ProvideLocalEnv(_ context.Context, kubeContext k8s.KubeContext, env k8s.Env, cEnv ClusterEnv) LocalEnv {
	result := overlayOSEnvVars(Env{})

	// if the ClusterEnv host is the same, use it to infer some properties
	if cEnv.Host == result.Host {
		result.IsOldMinikube = cEnv.IsOldMinikube
		result.BuildToKubeContexts = cEnv.BuildToKubeContexts
	}

	// TODO(milas): I'm fairly certain we're adding the `docker-desktop`
	//  kubecontext twice - the logic above should already have copied it
	// 	from the cluster env (this is harmless though because
	// 	Env::WillBuildToKubeContext still works fine)
	if env == k8s.EnvDockerDesktop && isDefaultHost(result) {
		result.BuildToKubeContexts = append(result.BuildToKubeContexts, string(kubeContext))
	}

	return LocalEnv(result)
}

func ProvideClusterEnv(ctx context.Context, kubeContext k8s.KubeContext, env k8s.Env, runtime container.Runtime, minikubeClient k8s.MinikubeClient) ClusterEnv {
	// start with an empty env, then populate with cluster-specific values if
	// available, and then potentially throw that all away if there are OS env
	// vars overriding those
	//
	// example: microk8s w/ Docker & no OS overrides -> use microk8s Docker socket
	//
	// example: minikube w/ Docker & DOCKER_HOST set via OS -> ignore result of
	// 	`minikube docker-env` and use OS values
	//
	// from a user standpoint, the behavior is:
	// 	- no values set at OS -> attempt to use cluster provided config
	//		this happens if you've done no extra config after cluster setup
	//	- values set at OS that differ from cluster config -> use OS values
	//		this is probably not as common, but an advanced setup could use
	//		e.g. a more powerful Docker host for builds but then run the images
	// 		on the local cluster
	// 	- values set at OS that match cluster config -> they're the same!
	//		this happens if you've run `eval (minikube docker-env)` in your
	// 		shell/profile so that you can use `docker` CLI with it too
	result := Env{}

	if runtime == container.RuntimeDocker {
		if env == k8s.EnvMinikube {
			// If we're running Minikube with a docker runtime, talk to Minikube's docker socket.
			envMap, ok, err := minikubeClient.DockerEnv(ctx)
			if err != nil {
				return ClusterEnv{Error: err}
			}

			if ok {
				result := Env{}
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

				result.IsOldMinikube = isOldMinikube(ctx, minikubeClient)
				result.BuildToKubeContexts = append(result.BuildToKubeContexts, string(kubeContext))
			}
		} else if env == k8s.EnvMicroK8s {
			// If we're running Microk8s with a docker runtime, talk to Microk8s's docker socket.
			result.Host = microK8sDockerHost
			result.BuildToKubeContexts = append(result.BuildToKubeContexts, string(kubeContext))
		}
	}

	// overlay OS values, potentially throwing away the cluster-provided config
	result = overlayOSEnvVars(Env{})
	if runtime == container.RuntimeDocker && isDefaultHost(result) {
		// currently both Docker Desktop + Rancher Desktop support running a
		// K8s cluster that shares the default Docker socket (as compared to
		// minikube/microk8s which maintain their own independent Docker socket)
		if env == k8s.EnvDockerDesktop || env == k8s.EnvRancherDesktop {
			result.BuildToKubeContexts = append(result.BuildToKubeContexts, string(kubeContext))
		}
	}

	return ClusterEnv(result)
}

func isOldMinikube(ctx context.Context, minikubeClient k8s.MinikubeClient) bool {
	v, err := minikubeClient.Version(ctx)
	if err != nil {
		logger.Get(ctx).Debugf("%v", err)
		return false
	}

	vParsed, err := semver.ParseTolerant(v)
	if err != nil {
		logger.Get(ctx).Debugf("Parsing minikube version: %v", err)
		return false
	}

	return minMinikubeVersionBuildkit.GTE(vParsed)
}

func isDefaultHost(e Env) bool {
	if e.Host == "" {
		return true
	}

	defaultHost, err := opts.ParseHost(true, "")
	if err != nil {
		return false
	}

	return e.Host == defaultHost
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
