package minikube

import "context"

type FakeClient struct {
	DockerEnvMap map[string]string
}

func (c FakeClient) DockerEnv(ctx context.Context) (map[string]string, error) {
	return c.DockerEnvMap, nil
}

var _ Client = FakeClient{}
