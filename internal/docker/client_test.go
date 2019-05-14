package docker

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/minikube"
)

type buildkitTestCase struct {
	v        types.Version
	expected bool
}

func TestSupportsBuildkit(t *testing.T) {
	cases := []buildkitTestCase{
		{types.Version{APIVersion: "1.37", Experimental: true}, false},
		{types.Version{APIVersion: "1.37", Experimental: false}, false},
		{types.Version{APIVersion: "1.38", Experimental: true}, true},
		{types.Version{APIVersion: "1.38", Experimental: false}, false},
		{types.Version{APIVersion: "1.39", Experimental: true}, true},
		{types.Version{APIVersion: "1.39", Experimental: false}, true},
		{types.Version{APIVersion: "1.40", Experimental: true}, true},
		{types.Version{APIVersion: "1.40", Experimental: false}, true},
		{types.Version{APIVersion: "garbage", Experimental: false}, false},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			assert.Equal(t, c.expected, SupportsBuildkit(c.v))
		})
	}
}

func TestSupported(t *testing.T) {
	cases := []buildkitTestCase{
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
	env      k8s.Env
	runtime  container.Runtime
	osEnv    map[string]string
	mkEnv    map[string]string
	expected Env
}

func TestProvideEnv(t *testing.T) {
	envVars := []string{
		"DOCKER_TLS_VERIFY",
		"DOCKER_HOST",
		"DOCKER_CERT_PATH",
		"DOCKER_API_VERSION",
	}

	cases := []provideEnvTestCase{
		{},
		{
			env: k8s.EnvUnknown,
			osEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			expected: Env{
				TLSVerify:  "1",
				Host:       "tcp://192.168.99.100:2376",
				CertPath:   "/home/nick/.minikube/certs",
				APIVersion: "1.35",
			},
		},
		{
			env:      k8s.EnvMicroK8s,
			runtime:  container.RuntimeDocker,
			expected: Env{Host: microK8sDockerHost},
		},
		{
			env:     k8s.EnvMicroK8s,
			runtime: container.RuntimeCrio,
		},
		{
			env:     k8s.EnvMinikube,
			runtime: container.RuntimeDocker,
			mkEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			expected: Env{
				TLSVerify:  "1",
				Host:       "tcp://192.168.99.100:2376",
				CertPath:   "/home/nick/.minikube/certs",
				APIVersion: "1.35",
			},
		},
		{
			env:     k8s.EnvMinikube,
			runtime: container.RuntimeDocker,
			mkEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			osEnv: map[string]string{
				"DOCKER_HOST": "registry.local:80",
			},
			expected: Env{
				Host: "registry.local:80",
			},
		},
		{
			env:     k8s.EnvMinikube,
			runtime: container.RuntimeCrio,
			mkEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "tcp://192.168.99.100:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			expected: Env{},
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			origEnv := map[string]string{}
			for _, k := range envVars {
				origEnv[k] = os.Getenv(k)
				os.Setenv(k, c.osEnv[k])
			}

			defer func() {
				for k := range c.osEnv {
					os.Setenv(k, origEnv[k])
				}
			}()

			mkClient := minikube.FakeClient{DockerEnvMap: c.mkEnv}
			actual, err := ProvideEnv(context.Background(), c.env, c.runtime, mkClient)
			if assert.NoError(t, err) {
				assert.Equal(t, c.expected, actual)
			}
		})
	}
}
