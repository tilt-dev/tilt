package k8s

import (
	"testing"

	"k8s.io/client-go/tools/clientcmd/api"
)

type expectedEnv struct {
	expected Env
	string
}

func TestEnvFromString(t *testing.T) {
	table := []expectedEnv{
		{EnvMinikube, "minikube"},
		{EnvDockerDesktop, "docker-for-desktop"},
		{EnvDockerDesktop, "docker-desktop"},
		{EnvGKE, "gke_blorg-dev_us-central1-b_blorg"},
		{EnvMicroK8s, "microk8s"},
		{EnvUnknown, "aws"},
		{EnvKIND, "kubernetes-admin@kind"},
		{EnvKIND, "kubernetes-admin@kind-1"},
	}

	for _, tt := range table {
		t.Run(tt.string, func(t *testing.T) {
			actual := EnvFromString(tt.string)
			if actual != tt.expected {
				t.Errorf("Expected %s, actual %s", tt.expected, actual)
			}
		})
	}
}

type expectedConfig struct {
	expected Env
	input    *api.Config
}

func TestEnvFromConfig(t *testing.T) {
	minikubeClusters := map[string]*api.Cluster{
		EnvMinikube: &api.Cluster{},
	}
	table := []expectedConfig{
		{EnvUnknown, &api.Config{CurrentContext: "aws"}},
		{EnvMinikube, &api.Config{CurrentContext: "minikube", Clusters: minikubeClusters}},
	}

	for _, tt := range table {
		t.Run(tt.input.CurrentContext, func(t *testing.T) {
			actual := EnvFromConfig(tt.input)
			if actual != tt.expected {
				t.Errorf("Expected %s, actual %s", tt.expected, actual)
			}
		})
	}
}
