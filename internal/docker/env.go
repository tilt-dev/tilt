package docker

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/cli/opts"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"github.com/tilt-dev/clusterid"

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

type DaemonClient interface {
	DaemonHost() string
}

// See notes on CreateClientOpts. These environment variables are standard docker env configs.
type Env struct {
	// The Docker API client.
	//
	// The Docker API builders are very complex, with lots of different interfaces
	// and concrete types, even though, underneath, they're all the same *client.Client.
	// We use DaemonClient here because that's what we need to compare envs.
	Client DaemonClient

	// Environment variables to inject into any subshell that uses this client.
	Environ []string

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
	return append([]string{}, e.Environ...)
}

func (e Env) DaemonHost() string {
	if e.Client == nil {
		return ""
	}
	return e.Client.DaemonHost()
}

type ClientCreator interface {
	FromCLI(ctx context.Context) (DaemonClient, error)
	FromEnvMap(envMap map[string]string) (DaemonClient, error)
}

type RealClientCreator struct{}

func (RealClientCreator) FromEnvMap(envMap map[string]string) (DaemonClient, error) {
	opts, err := CreateClientOpts(envMap)
	if err != nil {
		return nil, fmt.Errorf("configuring docker client: %v", err)
	}
	return client.NewClientWithOpts(opts...)
}

func (RealClientCreator) FromCLI(ctx context.Context) (DaemonClient, error) {
	cli, err := newDockerCli(ctx)
	if err != nil {
		return nil, err
	}

	client, ok := cli.Client().(*client.Client)
	if !ok {
		return nil, fmt.Errorf("unexpected docker client: %T", cli.Client())
	}
	return client, nil
}

// Creating a DockerCli is really the only way to get a DOCKER_CONTEXT-aware
// docker client.
func newDockerCli(ctx context.Context) (*command.DockerCli, error) {
	out := logger.Get(ctx).Writer(logger.InfoLvl)
	dockerCli, err := command.NewDockerCli(
		command.WithCombinedStreams(out))
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %v", err)
	}

	opts := cliflags.NewClientOptions()
	flagSet := pflag.NewFlagSet("docker", pflag.ContinueOnError)
	opts.InstallFlags(flagSet)
	opts.SetDefaultOptions(flagSet)
	err = dockerCli.Initialize(opts)
	if err != nil {
		return nil, fmt.Errorf("initializing docker client: %v", err)
	}

	// A hack to see if initialization failed.
	// https://github.com/docker/cli/issues/4489
	endpoint := dockerCli.DockerEndpoint()
	if endpoint.Host == "" {
		return nil, fmt.Errorf("initializing docker client: no valid endpoint")
	}
	return dockerCli, nil
}

// Tell wire to create two docker envs: one for the local CLI and one for the in-cluster CLI.
type ClusterEnv Env
type LocalEnv Env

func ProvideLocalEnv(
	ctx context.Context,
	creator ClientCreator,
	kubeContext k8s.KubeContext,
	product clusterid.Product,
	clusterEnv ClusterEnv,
) LocalEnv {
	result := Env{}
	client, err := creator.FromCLI(ctx)
	result.Client = client
	if err != nil {
		result.Error = err
	}

	// if the ClusterEnv host is the same, use it to infer some properties
	if Env(clusterEnv).DaemonHost() == result.DaemonHost() {
		result.IsOldMinikube = clusterEnv.IsOldMinikube
		result.BuildToKubeContexts = clusterEnv.BuildToKubeContexts
		result.Environ = clusterEnv.Environ
	}

	// TODO(milas): I'm fairly certain we're adding the `docker-desktop`
	//  kubecontext twice - the logic above should already have copied it
	// 	from the cluster env (this is harmless though because
	// 	Env::WillBuildToKubeContext still works fine)
	if product == clusterid.ProductDockerDesktop && isDefaultHost(result) {
		result.BuildToKubeContexts = append(result.BuildToKubeContexts, string(kubeContext))
	}

	return LocalEnv(result)
}

func ProvideClusterEnv(
	ctx context.Context,
	creator ClientCreator,
	kubeContext k8s.KubeContext,
	product clusterid.Product,
	kClient k8s.Client,
	minikubeClient k8s.MinikubeClient,
) ClusterEnv {
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
	env := Env{}

	hostOverride := os.Getenv("DOCKER_HOST")
	if hostOverride != "" {
		var err error
		hostOverride, err = opts.ParseHost(true, hostOverride)
		if err != nil {
			return ClusterEnv(Env{Error: errors.Wrap(err, "connecting to docker")})
		}
	}

	if product == clusterid.ProductMinikube && kClient.ContainerRuntime(ctx) == container.RuntimeDocker {
		// If we're running Minikube with a docker runtime, talk to Minikube's docker socket.
		envMap, ok, err := minikubeClient.DockerEnv(ctx)
		if err != nil {
			return ClusterEnv{Error: err}
		}

		if ok {
			d, err := creator.FromEnvMap(envMap)
			if err != nil {
				return ClusterEnv{Error: fmt.Errorf("connecting to minikube: %v", err)}
			}

			// Handle the case where people manually set DOCKER_HOST to minikube.
			if hostOverride == "" || hostOverride == d.DaemonHost() {
				env.IsOldMinikube = isOldMinikube(ctx, minikubeClient)
				env.BuildToKubeContexts = append(env.BuildToKubeContexts, string(kubeContext))
				env.Client = d
				for k, v := range envMap {
					env.Environ = append(env.Environ, fmt.Sprintf("%s=%s", k, v))
				}
				sort.Strings(env.Environ)
			}

		}
	}

	if env.Client == nil {
		client, err := creator.FromCLI(ctx)
		env.Client = client
		if err != nil {
			env.Error = err
		}
	}

	// some local Docker-based solutions expose their socket so we can build
	// direct to the K8s container runtime (eliminating the need for pushing
	// images)
	//
	// currently, we handle this by inspecting the Docker + K8s configs to see
	// if they're matched up, but we don't override the environmental Docker config
	if willBuildToKubeContext(ctx, product, kubeContext, env) &&
		kClient.ContainerRuntime(ctx) == container.RuntimeDocker {
		env.BuildToKubeContexts = append(env.BuildToKubeContexts, string(kubeContext))
	}

	return ClusterEnv(env)
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
	host := e.DaemonHost()
	isStandardHost :=
		// Check all the "standard" docker localhosts.
		host == "" ||

			// https://github.com/docker/cli/blob/a32cd16160f1b41c1c4ae7bee4dac929d1484e59/opts/hosts.go#L22
			host == "tcp://localhost:2375" ||
			host == "tcp://localhost:2376" ||
			host == "tcp://127.0.0.1:2375" ||
			host == "tcp://127.0.0.1:2376" ||

			// https://github.com/moby/moby/blob/master/client/client_windows.go#L4
			host == "npipe:////./pipe/docker_engine" ||

			// https://github.com/moby/moby/blob/master/client/client_unix.go#L6
			host == "unix:///var/run/docker.sock" ||

			// Docker Desktop for Linux - socket is in ~/.docker/desktop/docker.sock
			(strings.HasPrefix(host, "unix://") && strings.HasSuffix(host, "/.docker/desktop/docker.sock")) ||

			// Docker Desktop for Mac 4.13+ - socket is in ~/.docker/run/docker.sock
			(strings.HasPrefix(host, "unix://") && strings.HasSuffix(host, "/.docker/run/docker.sock")) ||

			// Docker Desktop for Windows 4.31+
			strings.HasPrefix(host, "npipe:////./pipe/dockerDesktop") ||

			// Rancher Desktop without admin access on Linux/Mac is in ~/.rd/docker.sock
			(strings.HasPrefix(host, "unix://") && strings.HasSuffix(host, "/.rd/docker.sock"))

	if isStandardHost {
		return true
	}

	defaultParseHost, err := opts.ParseHost(true, "")
	if err != nil {
		return false
	}

	return host == defaultParseHost

}

func willBuildToKubeContext(ctx context.Context, product clusterid.Product, kubeContext k8s.KubeContext, env Env) bool {
	switch product {
	case clusterid.ProductDockerDesktop:
		return isDefaultHost(env)
	case clusterid.ProductRancherDesktop:
		// N.B. Rancher Desktop creates a Docker socket at /var/run/docker.sock
		// (the same as Docker Desktop)
		return isDefaultHost(env)
	case clusterid.ProductColima:
		// Socket is stored in a directory named `.colima[-$profile]`
		// For example:
		//    colima default profile < 0.4 -> ~/.colima/docker.sock -> kubecontext colima
		//    colima "test" profile < 0.4 -> ~/.colima-test/docker.sock -> kubecontext colima-test
		//    colima default profile >= 0.4 -> ~/.colima/default/docker.sock -> kubecontext colima
		//    colima "test" profile >= 0.4 -> ~/.colima/test/docker.sock -> kubecontext colima-test
		//    colima linux profile >= 0.4 -> ~/.config/colima/default/docker.sock -> kubecontext colima
		//    colima linux "test" profile >= 0.4 -> ~/.config/colima/test/docker.sock -> kubecontext colima-test
		//
		// NOTE: ~ is used for legibility here; in practice, the paths MUST
		// be fully qualified!
		//
		// We look for the existence of the `/` in the path after the dir
		// to prevent mismatching Colima profiles: e.g. a KubeContext of
		// `colima-test` and `DOCKER_HOST=unix://~/.colima/docker.sock`
		// should NOT be considered as building to the context, as these
		// are two distinct Colima VMs/profiles. (This would almost always
		// be indicative of user error, but we respect the Docker + K8s
		// configs as provided to Tilt as-is. We emit a warning upon
		// detecting a likely misconfiguration here.)
		dockerColimaProfile := ""
		parts := strings.Split(env.DaemonHost(), "/.colima")
		if len(parts) == 1 {
			parts = strings.Split(env.DaemonHost(), "/.config/colima")
		}
		if len(parts) >= 2 {
			lastPart := parts[len(parts)-1]
			if strings.HasPrefix(lastPart, "-") {
				dockerColimaProfile = strings.Split(strings.TrimPrefix(lastPart, "-"), "/")[0]
			} else {
				moreParts := strings.Split(lastPart, "/")
				if len(moreParts) < 3 {
					dockerColimaProfile = "default"
				} else {
					dockerColimaProfile = moreParts[1]
				}
			}
		}

		if dockerColimaProfile == "" {
			logger.Get(ctx).Warnf("connected to Kubernetes running on Colima, but building on a non-Colima Docker socket")
			return false
		}

		kubeColimaProfile := ""
		if string(kubeContext) == "colima" {
			kubeColimaProfile = "default"
		} else {
			kubeColimaProfile = strings.TrimPrefix(string(kubeContext), "colima-")
		}
		if dockerColimaProfile != kubeColimaProfile {
			logger.Get(ctx).Warnf("connected to Kubernetes on Colima profile %s, but building on Docker on Colima profile %s",
				kubeColimaProfile, dockerColimaProfile)
			return false
		}
		return true
	case clusterid.ProductOrbstack:
		// Orbstack docker socket is $HOME/.orbstack/run/docker.sock
		host := env.DaemonHost()
		if strings.HasPrefix(host, "unix://") && strings.HasSuffix(host, "/.orbstack/run/docker.sock") {
			return true
		}
		logger.Get(ctx).Warnf("connected to Kubernetes running on Orbstack, but building on a non-Orbstack Docker socket")
		return false
	}
	return false
}
