package cluster

import (
	"context"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
)

func FakeKubernetesClientOrError(client k8s.Client, err error) KubernetesClientFactory {
	return KubernetesClientFunc(func(
		_ context.Context,
		_ k8s.KubeContextOverride,
		_ k8s.NamespaceOverride,
	) (k8s.Client, error) {
		if err != nil {
			return nil, err
		}
		return client, nil
	})
}

func FakeDockerClientOrError(client docker.Client, err error) DockerClientFactory {
	return DockerClientFunc(func(_ context.Context, _ docker.Env) (docker.Client, error) {
		if err != nil {
			return nil, err
		}
		return client, nil
	})
}
