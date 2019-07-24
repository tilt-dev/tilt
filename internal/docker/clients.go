package docker

import (
	"context"
)

type LocalClient Client
type ClusterClient Client

func ProvideClusterCli(ctx context.Context, lEnv LocalEnv, cEnv ClusterEnv, lClient LocalClient) (ClusterClient, error) {
	// If the Cluster Env and the LocalEnv are the same, we can re-use the cluster
	// client as a local client.
	if Env(lEnv) == Env(cEnv) {
		return ClusterClient(lClient), nil
	}
	result, err := NewDockerClient(ctx, Env(cEnv))
	return ClusterClient(result), err
}

func ProvideLocalCli(ctx context.Context, lEnv LocalEnv) (LocalClient, error) {
	result, err := NewDockerClient(ctx, Env(lEnv))
	return LocalClient(result), err
}
