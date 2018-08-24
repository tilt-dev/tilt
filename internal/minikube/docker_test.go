package minikube

import "testing"

func TestDockerEnv(t *testing.T) {
	output := []byte(`
export DOCKER_TLS_VERIFY="1"
export DOCKER_HOST="tcp://192.168.99.100:2376"
export DOCKER_CERT_PATH="/home/nick/.minikube/certs"
export DOCKER_API_VERSION="1.35"
# Run this command to configure your shell:
# eval $(minikube docker-env)
`)

	env, err := dockerEnvFromOutput(output)
	if err != nil {
		t.Fatal(err)
	}

	if len(env) != 4 ||
		env["DOCKER_TLS_VERIFY"] != "1" ||
		env["DOCKER_HOST"] != "tcp://192.168.99.100:2376" ||
		env["DOCKER_CERT_PATH"] != "/home/nick/.minikube/certs" ||
		env["DOCKER_API_VERSION"] != "1.35" {
		t.Errorf("Unexpected env: %+v", env)
	}
}
