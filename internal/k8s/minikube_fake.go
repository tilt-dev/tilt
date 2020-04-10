package k8s

import "context"

type FakeMinikube struct {
	FakeVersion  string
	DockerEnvMap map[string]string
}

func (c FakeMinikube) Version(ctx context.Context) (string, error) {
	return c.FakeVersion, nil
}

func (c FakeMinikube) DockerEnv(ctx context.Context) (map[string]string, error) {
	return c.DockerEnvMap, nil
}

func (c FakeMinikube) NodeIP(ctx context.Context) (NodeIP, error) {
	return "", nil
}

var _ MinikubeClient = FakeMinikube{}
