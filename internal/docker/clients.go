package docker

import (
	"context"
)

type LocalClient Client
type ClusterClient Client

func ProvideClusterCli(ctx context.Context, lEnv LocalEnv, cEnv ClusterEnv, lClient LocalClient) (ClusterClient, error) {
	// If the Cluster Env and the LocalEnv are the same, we can re-use the cluster
	// client as a local client.
	var cClient ClusterClient
	if Env(lEnv) == Env(cEnv) {
		cClient = ClusterClient(lClient)
	} else {
		cClient = NewDockerClient(ctx, Env(cEnv))
	}

	// The LocalClient is the docker server from docker env variables.
	// The ClusterClient is the docker server from kubectl configs.
	// If neither of them work, we can fail on startup.
	// If only one of them works, we have to wait until Tiltfile load to find out
	// which one we need.
	err1 := cClient.CheckConnected()
	err2 := lClient.CheckConnected()
	if err1 != nil && err2 != nil {
		return nil, err1
	}
	return cClient, nil
}

func ProvideLocalCli(ctx context.Context, lEnv LocalEnv) LocalClient {
	return LocalClient(NewDockerClient(ctx, Env(lEnv)))
}
