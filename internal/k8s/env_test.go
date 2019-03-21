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
	minikubeContexts := map[string]*api.Context{
		"minikube": &api.Context{
			Cluster: "minikube",
		},
	}
	dockerDesktopContexts := map[string]*api.Context{
		"docker-for-desktop": &api.Context{
			Cluster: "docker-for-desktop-cluster",
		},
	}
	gkeContexts := map[string]*api.Context{
		"gke_blorg-dev_us-central1-b_blorg": &api.Context{
			Cluster: "gke_blorg-dev_us-central1-b_blorg",
		},
	}
	kindContexts := map[string]*api.Context{
		"kubernetes-admin@kind-1": &api.Context{
			Cluster: "kind",
		},
	}
	microK8sContexts := map[string]*api.Context{
		"microk8s": &api.Context{
			Cluster: "microk8s-cluster",
		},
	}
	table := []expectedConfig{
		{EnvUnknown, &api.Config{CurrentContext: "aws"}},
		{EnvMinikube, &api.Config{CurrentContext: "minikube", Contexts: minikubeContexts}},
		{EnvDockerDesktop, &api.Config{CurrentContext: "docker-for-desktop", Contexts: dockerDesktopContexts}},
		{EnvGKE, &api.Config{CurrentContext: "gke_blorg-dev_us-central1-b_blorg", Contexts: gkeContexts}},
		{EnvKIND, &api.Config{CurrentContext: "kubernetes-admin@kind-1", Contexts: kindContexts}},
		{EnvMicroK8s, &api.Config{CurrentContext: "microk8s", Contexts: microK8sContexts}},
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
