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
	env      k8s.Env
	expected bool
}

func TestSupportsBuildkit(t *testing.T) {
	cases := []buildkitTestCase{
		{types.Version{APIVersion: "1.37", Experimental: true}, k8s.EnvGKE, false},
		{types.Version{APIVersion: "1.37", Experimental: false}, k8s.EnvGKE, false},
		{types.Version{APIVersion: "1.38", Experimental: true}, k8s.EnvGKE, true},
		{types.Version{APIVersion: "1.38", Experimental: false}, k8s.EnvGKE, false},
		{types.Version{APIVersion: "1.39", Experimental: true}, k8s.EnvGKE, true},
		{types.Version{APIVersion: "1.39", Experimental: false}, k8s.EnvGKE, true},
		{types.Version{APIVersion: "1.40", Experimental: true}, k8s.EnvGKE, true},
		{types.Version{APIVersion: "1.40", Experimental: false}, k8s.EnvGKE, true},
		{types.Version{APIVersion: "garbage", Experimental: false}, k8s.EnvGKE, false},
		{types.Version{APIVersion: "1.39", Experimental: true}, k8s.EnvMinikube, false},
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
		{"1.37", "1", types.BuilderBuildKit},
		{"1.40", "", types.BuilderBuildKit},
		{"1.40", "0", types.BuilderV1},
		{"1.40", "1", types.BuilderBuildKit},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			os.Setenv("DOCKER_BUILDKIT", c.bkEnv)
			defer os.Setenv("DOCKER_BUILDKIT", "")

			v, err := ProvideDockerBuilderVersion(
				types.Version{APIVersion: c.v}, k8s.EnvGKE)
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
				"DOCKER_HOST": "tcp://registry.local:80",
			},
			expected: Env{
				Host: "tcp://registry.local:80",
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
		{
			env: k8s.EnvUnknown,
			osEnv: map[string]string{
				"DOCKER_TLS_VERIFY":  "1",
				"DOCKER_HOST":        "localhost:2376",
				"DOCKER_CERT_PATH":   "/home/nick/.minikube/certs",
				"DOCKER_API_VERSION": "1.35",
			},
			expected: Env{
				TLSVerify:  "1",
				Host:       "tcp://localhost:2376",
				CertPath:   "/home/nick/.minikube/certs",
				APIVersion: "1.35",
			},
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
