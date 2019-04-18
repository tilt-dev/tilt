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
	CommandsRunCount             int
	UpdateContainerHotReload     bool
	ClosedCount                  int
	UpdateContainerErrorToReturn error
	PodID                        k8s.PodID
	Namespace                    k8s.Namespace
}

var _ SyncletClient = &FakeSyncletClient{}

func NewFakeSyncletClient() *FakeSyncletClient {
	return &FakeSyncletClient{}
}

func (c *FakeSyncletClient) UpdateContainer(ctx context.Context, containerID container.ID,
	tarArchive []byte, filesToDelete []string, commands []model.Cmd, hotReload bool) error {
	if c.UpdateContainerErrorToReturn != nil {
		ret := c.UpdateContainerErrorToReturn
		c.UpdateContainerErrorToReturn = nil
		return ret
	}
	c.UpdateContainerCount += 1
	c.UpdateContainerHotReload = hotReload
	c.CommandsRunCount += len(commands)
	return nil
}

func (c *FakeSyncletClient) ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (container.ID, error) {
	return container.ID("foobar"), nil
}

func (c *FakeSyncletClient) Close() error {
	c.ClosedCount++
	return nil
}
