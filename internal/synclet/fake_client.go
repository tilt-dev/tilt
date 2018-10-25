package synclet

import (
	"context"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

// NOTE(maia): idk if we even need this -- maybe what we want is a real synclet client
// with a fake docker client inside it. But ¯\_(ツ)_/¯
type FakeSyncletClient struct {
	UpdateContainerCount         int
	ClosedCount                  int
	UpdateContainerErrorToReturn error
	PodID                        k8s.PodID
	Namespace                    k8s.Namespace
}

var _ SyncletClient = &FakeSyncletClient{}

func NewFakeSyncletClient() *FakeSyncletClient {
	return &FakeSyncletClient{}
}

func (c *FakeSyncletClient) UpdateContainer(ctx context.Context, containerID container.ContainerID,
	tarArchive []byte, filesToDelete []string, commands []model.Cmd) error {
	if c.UpdateContainerErrorToReturn != nil {
		ret := c.UpdateContainerErrorToReturn
		c.UpdateContainerErrorToReturn = nil
		return ret
	}
	c.UpdateContainerCount += 1
	return nil
}

func (c *FakeSyncletClient) ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (container.ContainerID, error) {
	return container.ContainerID("foobar"), nil
}

func (c *FakeSyncletClient) Close() error {
	c.ClosedCount++
	return nil
}
