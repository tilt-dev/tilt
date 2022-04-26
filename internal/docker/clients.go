package docker

import (
	"context"
)

type LocalClient Client
type ClusterClient Client

// The LocalClient is the docker server from docker env variables.
// The ClusterClient is the docker server from kubectl configs.
//
// We may need both or just one or neither, depending on what options the
// Tiltfile has set to drive the build
func ProvideClusterCli(ctx context.Context, lEnv LocalEnv, cEnv ClusterEnv, lClient LocalClient) (ClusterClient, error) {
	// If the Cluster Env and the LocalEnv talk to the same daemon,
	// we can re-use the cluster client as a local client.
	var cClient ClusterClient
	if Env(lEnv).DaemonHost() == Env(cEnv).DaemonHost() {
		cClient = ClusterClient(lClient)
	} else {
		cClient = NewDockerClient(ctx, Env(cEnv))
	}

	return cClient, nil
}

func ProvideLocalCli(ctx context.Context, lEnv LocalEnv) LocalClient {
	return LocalClient(NewDockerClient(ctx, Env(lEnv)))
}
