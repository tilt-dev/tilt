package docker

import (
	"context"
	"io"
	"sync"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/pkg/model"
)

// A Cli implementation that lets us switch back and forth between a local
// Docker instance and one that lives in our K8s cluster.

type switchCli struct {
	localCli   LocalClient
	clusterCli ClusterClient
	orc        model.Orchestrator
	mu         sync.Mutex
}

func ProvideSwitchCli(clusterCli ClusterClient, localCli LocalClient) *switchCli {
	return &switchCli{
		localCli:   localCli,
		clusterCli: clusterCli,
		orc:        model.OrchestratorK8s,
	}
}

func (c *switchCli) client() Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.orc == model.OrchestratorK8s {
		return c.clusterCli
	}
	return c.localCli
}

func (c *switchCli) SetOrchestrator(orc model.Orchestrator) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.orc = orc
}
func (c *switchCli) CheckConnected() error {
	return c.client().CheckConnected()
}
func (c *switchCli) Env() Env {
	return c.client().Env()
}
func (c *switchCli) BuilderVersion() types.BuilderVersion {
	return c.client().BuilderVersion()
}
func (c *switchCli) ServerVersion() types.Version {
	return c.client().ServerVersion()
}
func (c *switchCli) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	return c.client().ContainerList(ctx, options)
}
func (c *switchCli) ContainerRestartNoWait(ctx context.Context, containerID string) error {
	return c.client().ContainerRestartNoWait(ctx, containerID)
}
func (c *switchCli) CopyToContainerRoot(ctx context.Context, container string, content io.Reader) error {
	return c.client().CopyToContainerRoot(ctx, container, content)
}
func (c *switchCli) ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, out io.Writer) error {
	return c.client().ExecInContainer(ctx, cID, cmd, out)
}
func (c *switchCli) ImagePush(ctx context.Context, ref reference.NamedTagged) (io.ReadCloser, error) {
	return c.client().ImagePush(ctx, ref)
}
func (c *switchCli) ImageBuild(ctx context.Context, buildContext io.Reader, options BuildOptions) (types.ImageBuildResponse, error) {
	return c.client().ImageBuild(ctx, buildContext, options)
}
func (c *switchCli) ImageTag(ctx context.Context, source, target string) error {
	return c.client().ImageTag(ctx, source, target)
}
func (c *switchCli) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	return c.client().ImageInspectWithRaw(ctx, imageID)
}
func (c *switchCli) ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error) {
	return c.client().ImageList(ctx, options)
}
func (c *switchCli) ImageRemove(ctx context.Context, imageID string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	return c.client().ImageRemove(ctx, imageID, options)
}
func (c *switchCli) NewVersionError(APIrequired, feature string) error {
	return c.client().NewVersionError(APIrequired, feature)
}
func (c *switchCli) BuildCachePrune(ctx context.Context, opts types.BuildCachePruneOptions) (*types.BuildCachePruneReport, error) {
	return c.client().BuildCachePrune(ctx, opts)
}
func (c *switchCli) ContainersPrune(ctx context.Context, pruneFilters filters.Args) (types.ContainersPruneReport, error) {
	return c.client().ContainersPrune(ctx, pruneFilters)
}

var _ Client = &switchCli{}
