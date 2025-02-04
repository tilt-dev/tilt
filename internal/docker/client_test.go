package docker

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
)

type buildkitTestCase struct {
	v        types.Version
	env      Env
	expected bool
}

func TestSupportsBuildkit(t *testing.T) {
	cases := []buildkitTestCase{
		{types.Version{APIVersion: "1.37", Experimental: true}, Env{}, false},
		{types.Version{APIVersion: "1.37", Experimental: false}, Env{}, false},
		{types.Version{APIVersion: "1.38", Experimental: true}, Env{}, true},
		{types.Version{APIVersion: "1.38", Experimental: false}, Env{}, false},
		{types.Version{APIVersion: "1.39", Experimental: true}, Env{}, true},
		{types.Version{APIVersion: "1.39", Experimental: false}, Env{}, true},
		{types.Version{APIVersion: "1.40", Experimental: true}, Env{}, true},
		{types.Version{APIVersion: "1.40", Experimental: false}, Env{}, true},
		{types.Version{APIVersion: "garbage", Experimental: false}, Env{}, false},
		{types.Version{APIVersion: "1.39", Experimental: true}, Env{IsOldMinikube: true}, false},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			assert.Equal(t, c.expected, SupportsBuildkit(c.v, c.env))
		})
	}
}

type builderVersionTestCase struct {
	v        string
	bkEnv    string
	expected types.BuilderVersion
}

func TestProvideBuilderVersion(t *testing.T) {
	cases := []builderVersionTestCase{
		{"1.37", "", types.BuilderV1},
		{"1.37", "0", types.BuilderV1},
		{"1.37", "1", types.BuilderV1},
		{"1.40", "", types.BuilderBuildKit},
		{"1.40", "0", types.BuilderV1},
		{"1.40", "1", types.BuilderBuildKit},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			t.Setenv("DOCKER_BUILDKIT", c.bkEnv)

			v, err := getDockerBuilderVersion(
				types.Version{APIVersion: c.v}, Env{})
			assert.NoError(t, err)
			assert.Equal(t, c.expected, v)
		})
	}
}

type versionTestCase struct {
	v        types.Version
	expected bool
}

func TestSupported(t *testing.T) {
	cases := []versionTestCase{
		{types.Version{APIVersion: "1.22"}, false},
		{types.Version{APIVersion: "1.23"}, true},
		{types.Version{APIVersion: "1.39"}, true},
		{types.Version{APIVersion: "1.40"}, true},
		{types.Version{APIVersion: "garbage"}, false},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			assert.Equal(t, c.expected, SupportedVersion(c.v))
		})
	}
}

type provideEnvTestCase struct {
	env             clusterid.Product
	runtime         container.Runtime
	minikubeV       string
	osEnv           map[string]string
	mkEnv           map[string]string
	expectedCluster Env
	expectedLocal   Env
}

type hostClient struct {
	Host string
}

func (c hostClient) DaemonHost() string { return c.Host }

type fakeClientCreator struct {
}

func (c fakeClientCreator) FromCLI(ctx context.Context) (DaemonClient, error) {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}
	return hostClient{Host: host}, nil
}
func (c fakeClientCreator) FromEnvMap(envMap map[string]string) (DaemonClient, error) {
	host := envMap["DOCKER_HOST"]
	return hostClient{Host: host}, nil
}

func TestProvideClusterProduct(t *testing.T) {
	envVars := []string{
		"DOCKER_TLS_VERIFY",
		"DOCKER_HOST",
		"DOCKER_CERT_PATH",
		"DOCKER_API_VERSION",
	}

	cases := []provideEnvTestCase{
		{
			expectedCluster: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
			expectedLocal: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
		},
		{
			env: clusterid.ProductUnknown,
			osEnv: map[string]string{
				"DOCKER_HOST": "tcp://192.168.99.100:2376",
			},
			expectedCluster: Env{
				Client: hostClient{Host: "tcp://192.168.99.100:2376"},
			},
			expectedLocal: Env{
				Client: hostClient{Host: "tcp://192.168.99.100:2376"},
			},
		},
		{
			env:     clusterid.ProductMicroK8s,
			runtime: container.RuntimeCrio,
			expectedCluster: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
			expectedLocal: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
		},
		{
			env:     clusterid.ProductMinikube,
			runtime: container.RuntimeDocker,
			mkEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			expectedCluster: Env{
				Client: hostClient{Host: "tcp://192.168.99.100:2376"},
				Environ: []string{
					"DOCKER_API_VERSION=1.35",
					"DOCKER_CERT_PATH=/home/nick/.minikube/certs",
					"DOCKER_HOST=tcp://192.168.99.100:2376",
					"DOCKER_TLS_VERIFY=1",
				},
				IsOldMinikube:       true,
				BuildToKubeContexts: []string{"minikube-me"},
			},
			expectedLocal: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
		},
		{
			env:       clusterid.ProductMinikube,
			runtime:   container.RuntimeDocker,
			minikubeV: "1.8.2",
			mkEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			expectedCluster: Env{
				Client: hostClient{Host: "tcp://192.168.99.100:2376"},
				Environ: []string{
					"DOCKER_API_VERSION=1.35",
					"DOCKER_CERT_PATH=/home/nick/.minikube/certs",
					"DOCKER_HOST=tcp://192.168.99.100:2376",
					"DOCKER_TLS_VERIFY=1",
				},
				BuildToKubeContexts: []string{"minikube-me"},
			},
			expectedLocal: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
		},
		{
			env:     clusterid.ProductMinikube,
			runtime: container.RuntimeDocker,
			mkEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			osEnv: map[string]string{
				"DOCKER_HOST": "tcp://registry.local:80",
			},
			expectedCluster: Env{
				Client: hostClient{Host: "tcp://registry.local:80"},
			},
			expectedLocal: Env{
				Client: hostClient{Host: "tcp://registry.local:80"},
			},
		},
		{
			// Test the case where the user has already run
			// eval $(minikube docker-env)
			env:     clusterid.ProductMinikube,
			runtime: container.RuntimeDocker,
			mkEnv: map[string]string{
				"DOCKER_TLS_VERIFY": "1",
				"DOCKER_HOST":       "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":  "/home/nick/.minikube/certs",
			},
			osEnv: map[string]string{
				"DOCKER_TLS_VERIFY": "1",
				"DOCKER_HOST":       "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":  "/home/nick/.minikube/certs",
			},
			expectedCluster: Env{
				Client:              hostClient{Host: "tcp://192.168.99.100:2376"},
				Environ:             []string{"DOCKER_CERT_PATH=/home/nick/.minikube/certs", "DOCKER_HOST=tcp://192.168.99.100:2376", "DOCKER_TLS_VERIFY=1"},
				IsOldMinikube:       true,
				BuildToKubeContexts: []string{"minikube-me"},
			},
			expectedLocal: Env{
				Client:              hostClient{Host: "tcp://192.168.99.100:2376"},
				Environ:             []string{"DOCKER_CERT_PATH=/home/nick/.minikube/certs", "DOCKER_HOST=tcp://192.168.99.100:2376", "DOCKER_TLS_VERIFY=1"},
				IsOldMinikube:       true,
				BuildToKubeContexts: []string{"minikube-me"},
			},
		},
		{
			env:     clusterid.ProductMinikube,
			runtime: container.RuntimeCrio,
			mkEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			expectedCluster: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
			expectedLocal: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
		},
		{
			env: clusterid.ProductUnknown,
			osEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "localhost:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			expectedCluster: Env{
				Client: hostClient{Host: "localhost:2376"},
			},
			expectedLocal: Env{
				Client: hostClient{Host: "localhost:2376"},
			},
		},
		{
			env:     clusterid.ProductDockerDesktop,
			runtime: container.RuntimeDocker,
			expectedCluster: Env{
				Client:              hostClient{Host: "unix:///var/run/docker.sock"},
				BuildToKubeContexts: []string{"docker-desktop-me"},
			},
			expectedLocal: Env{
				Client:              hostClient{Host: "unix:///var/run/docker.sock"},
				BuildToKubeContexts: []string{"docker-desktop-me", "docker-desktop-me"},
			},
		},
		{
			env:     clusterid.ProductDockerDesktop,
			runtime: container.RuntimeDocker,
			// Set DOCKER_HOST here to mimic docker client discovering socket from desktop-linux context
			osEnv: map[string]string{
				"DOCKER_HOST": "unix:///home/tilt/.docker/desktop/docker.sock",
			},
			expectedCluster: Env{
				Client:              hostClient{Host: "unix:///home/tilt/.docker/desktop/docker.sock"},
				BuildToKubeContexts: []string{"docker-desktop-me"},
			},
			expectedLocal: Env{
				Client:              hostClient{Host: "unix:///home/tilt/.docker/desktop/docker.sock"},
				BuildToKubeContexts: []string{"docker-desktop-me", "docker-desktop-me"},
			},
		},
		{
			env:     clusterid.ProductDockerDesktop,
			runtime: container.RuntimeDocker,
			osEnv: map[string]string{
				"DOCKER_HOST": "unix:///home/tilt/.docker/run/docker.sock",
			},
			expectedCluster: Env{
				Client:              hostClient{Host: "unix:///home/tilt/.docker/run/docker.sock"},
				BuildToKubeContexts: []string{"docker-desktop-me"},
			},
			expectedLocal: Env{
				Client:              hostClient{Host: "unix:///home/tilt/.docker/run/docker.sock"},
				BuildToKubeContexts: []string{"docker-desktop-me", "docker-desktop-me"},
			},
		},
		{
			env:     clusterid.ProductRancherDesktop,
			runtime: container.RuntimeDocker,
			expectedCluster: Env{
				Client:              hostClient{Host: "unix:///var/run/docker.sock"},
				BuildToKubeContexts: []string{"rancher-desktop-me"},
			},
			expectedLocal: Env{
				Client:              hostClient{Host: "unix:///var/run/docker.sock"},
				BuildToKubeContexts: []string{"rancher-desktop-me"},
			},
		},
		{
			env:     clusterid.ProductRancherDesktop,
			runtime: container.RuntimeContainerd,
			expectedCluster: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
			expectedLocal: Env{
				Client: hostClient{Host: "unix:///var/run/docker.sock"},
			},
		},
		{
			env:     clusterid.ProductRancherDesktop,
			runtime: container.RuntimeDocker,
			// Set DOCKER_HOST here to mimic rancher-desktop docker context in non-admin mode
			osEnv: map[string]string{
				"DOCKER_HOST": "unix:///Users/tilt/.rd/docker.sock",
			},
			expectedCluster: Env{
				Client:              hostClient{Host: "unix:///Users/tilt/.rd/docker.sock"},
				BuildToKubeContexts: []string{"rancher-desktop-me"},
			},
			expectedLocal: Env{
				Client:              hostClient{Host: "unix:///Users/tilt/.rd/docker.sock"},
				BuildToKubeContexts: []string{"rancher-desktop-me"},
			},
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			for _, k := range envVars {
				t.Setenv(k, c.osEnv[k])
			}

			minikubeV := c.minikubeV
			if minikubeV == "" {
				minikubeV = "1.3.0" // an Old version
			}

			mkClient := k8s.FakeMinikube{DockerEnvMap: c.mkEnv, FakeVersion: minikubeV}
			kubeContext := k8s.KubeContext(fmt.Sprintf("%s-me", c.env))
			kCli := k8s.NewFakeK8sClient(t)
			kCli.Runtime = c.runtime
			cluster := ProvideClusterEnv(context.Background(), fakeClientCreator{}, kubeContext, c.env, kCli, mkClient)
			assert.Equal(t, c.expectedCluster, Env(cluster))

			local := ProvideLocalEnv(context.Background(), fakeClientCreator{}, kubeContext, c.env, cluster)
			assert.Equal(t, c.expectedLocal, Env(local))
		})
	}
}
