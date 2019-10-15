package docker

import (
	"context"
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/pkg/model"
)

// A docker client that returns errors on every method call.
// Useful when the client failed to init, but we don't have enough
// info at init time to tell if anyone is going to use it.

type explodingClient struct {
	err error
}

func newExplodingClient(err error) explodingClient {
	return explodingClient{err: err}
}

func (c explodingClient) SetOrchestrator(orc model.Orchestrator) {
}
func (c explodingClient) CheckConnected() error {
	return c.err
}
func (c explodingClient) Env() Env {
	return Env{}
}
func (c explodingClient) BuilderVersion() types.BuilderVersion {
	return types.BuilderVersion("")
}
func (c explodingClient) ServerVersion() types.Version {
	return types.Version{}
}
func (c explodingClient) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	return nil, c.err
}
func (c explodingClient) ContainerRestartNoWait(ctx context.Context, containerID string) error {
	return c.err
}
func (c explodingClient) CopyToContainerRoot(ctx context.Context, container string, content io.Reader) error {
	return c.err
}
func (c explodingClient) ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, out io.Writer) error {
	return c.err
}
func (c explodingClient) ImagePush(ctx context.Context, ref reference.NamedTagged) (io.ReadCloser, error) {
	return nil, c.err
}
func (c explodingClient) ImageBuild(ctx context.Context, buildContext io.Reader, options BuildOptions) (types.ImageBuildResponse, error) {
	return types.ImageBuildResponse{}, c.err
}
func (c explodingClient) ImageTag(ctx context.Context, source, target string) error {
	return c.err
}
func (c explodingClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	return types.ImageInspect{}, nil, c.err
}
func (c explodingClient) ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error) {
	return nil, c.err
}
func (c explodingClient) ImageRemove(ctx context.Context, imageID string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	return nil, c.err
}
func (c explodingClient) NewVersionError(APIrequired, feature string) error {
	return c.err
}
func (c explodingClient) BuildCachePrune(ctx context.Context, opts types.BuildCachePruneOptions) (*types.BuildCachePruneReport, error) {
	return nil, c.err
}
func (c explodingClient) ContainersPrune(ctx context.Context, pruneFilters filters.Args) (types.ContainersPruneReport, error) {
	return types.ContainersPruneReport{}, c.err
}

var _ Client = &explodingClient{}
