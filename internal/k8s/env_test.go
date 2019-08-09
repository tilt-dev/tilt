package k8s

import (
	"testing"

	"k8s.io/client-go/tools/clientcmd/api"
)

type expectedEnv struct {
	expected Env
	string
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
	dockerDesktopEdgeContexts := map[string]*api.Context{
		"docker-for-desktop": &api.Context{
			Cluster: "docker-desktop",
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
		{EnvDockerDesktop, &api.Config{CurrentContext: "docker-for-desktop", Contexts: dockerDesktopEdgeContexts}},
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
