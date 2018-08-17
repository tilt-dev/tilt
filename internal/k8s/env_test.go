package k8s

import "testing"

type expectedEnv struct {
	expected Env
	string
}

func TestEnvFromString(t *testing.T) {
	table := []expectedEnv{
		{EnvMinikube, "minikube"},
		{EnvDockerDesktop, "docker-for-desktop"},
		{EnvGKE, "gke_blorg-dev_us-central1-b_blorg"},
		{EnvUnknown, "aws"},
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
