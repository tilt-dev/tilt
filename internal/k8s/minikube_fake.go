package k8s

import "context"

type FakeMinkube struct {
	DockerEnvMap map[string]string
}

func (c FakeMinkube) DockerEnv(ctx context.Context) (map[string]string, error) {
	return c.DockerEnvMap, nil
}

func (c FakeMinkube) NodeIP(ctx context.Context) (NodeIP, error) {
	return "", nil
}

var _ MinikubeClient = FakeMinkube{}
