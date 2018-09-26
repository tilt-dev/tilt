package synclet

import (
	"context"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

// NOTE(maia): idk if we even need this -- maybe what we want is a real synclet client
// with a fake docker client inside it. But ¯\_(ツ)_/¯
type FakeSyncletClient struct {
	UpdateContainerCount int
	ClosedCount          int
	ErrorToReturn        error
}

var _ SyncletClient = &FakeSyncletClient{}

func NewFakeSyncletClient() *FakeSyncletClient {
	return &FakeSyncletClient{}
}

func (c *FakeSyncletClient) UpdateContainer(ctx context.Context, containerID k8s.ContainerID,
	tarArchive []byte, filesToDelete []string, commands []model.Cmd) error {
	if c.ErrorToReturn != nil {
		ret := c.ErrorToReturn
		c.ErrorToReturn = nil
		return ret
	}
	c.UpdateContainerCount += 1
	return nil
}

func (c *FakeSyncletClient) ContainerIDForPod(ctx context.Context, podID k8s.PodID, imageID reference.NamedTagged) (k8s.ContainerID, error) {
	if c.ErrorToReturn != nil {
		ret := c.ErrorToReturn
		c.ErrorToReturn = nil
		return k8s.ContainerID(""), ret
	}

	return k8s.ContainerID("foobar"), nil
}

func (c *FakeSyncletClient) Close() error {
	c.ClosedCount++
	return nil
}
