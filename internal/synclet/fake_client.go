package synclet

import (
	context "context"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

// NOTE(maia): idk if we even need this -- maybe what we want is a real synclet client
// with a fake docker client inside it. But ¯\_(ツ)_/¯
type FakeSyncletClient struct{}

var _ SyncletClient = &FakeSyncletClient{}

func NewFakeSyncletClient() *FakeSyncletClient {
	return &FakeSyncletClient{}
}
func (c *FakeSyncletClient) UpdateContainer(ctx context.Context, containerId string, tarArchive []byte,
	filesToDelete []string, commands []model.Cmd) error {
	return nil
}

func (c *FakeSyncletClient) GetContainerIdForPod(ctx context.Context, podId k8s.PodID) (k8s.ContainerID, error) {
	return "", nil
}
