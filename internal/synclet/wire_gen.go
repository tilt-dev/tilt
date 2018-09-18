// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package synclet

import (
	context "context"

	wmdocker "github.com/windmilleng/tilt/internal/docker"
	k8s "github.com/windmilleng/tilt/internal/k8s"
)

// Injectors from wire.go:

func WireSynclet(ctx context.Context, env k8s.Env) (*Synclet, error) {
	dockerCli, err := wmdocker.DefaultDockerClient(ctx, env)
	if err != nil {
		return nil, err
	}
	synclet := NewSynclet(dockerCli)
	return synclet, nil
}
