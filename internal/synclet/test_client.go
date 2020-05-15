package synclet

import (
	"context"

	"github.com/tilt-dev/tilt/internal/docker"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/model"
)

type TestSyncletClient struct {
	synclet *Synclet

	// To make sure that any Docker calls we're asserting on
	// came via the synclet (and not through some other path
	// to the same Docker client)
	UpdateContainerCount int

	// To make sure client was configured correctly by SyncletClientManager
	PodID     k8s.PodID
	Namespace k8s.Namespace
}

var _ SyncletClient = &TestSyncletClient{}

func NewTestSyncletClient(dCli docker.Client) *TestSyncletClient {
	return &TestSyncletClient{synclet: NewSynclet(dCli)}
}

func (c *TestSyncletClient) UpdateContainer(ctx context.Context, containerID container.ID,
	tarArchive []byte, filesToDelete []string, commands []model.Cmd, hotReload bool) error {
	c.UpdateContainerCount += 1
	return c.synclet.UpdateContainer(ctx, containerID, tarArchive, filesToDelete, commands, hotReload)
}

func (c *TestSyncletClient) Close() error {
	return nil
}
