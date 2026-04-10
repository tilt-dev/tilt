package docker

import (
	"context"
	"io"

	"github.com/distribution/reference"
	typesbuild "github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
	"golang.org/x/sync/errgroup"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/model"
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
func (c explodingClient) ForOrchestrator(orc model.Orchestrator) Client {
	return c
}
func (c explodingClient) CheckConnected() error {
	return c.err
}
func (c explodingClient) Env() Env {
	return Env{}
}
func (c explodingClient) BuilderVersion(ctx context.Context) (typesbuild.BuilderVersion, error) {
	return typesbuild.BuilderV1, c.err
}
func (c explodingClient) ServerVersion(ctx context.Context) (client.ServerVersionResult, error) {
	return client.ServerVersionResult{}, c.err
}
func (c explodingClient) ContainerLogs(ctx context.Context, containerID string, options client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	return nil, c.err
}
func (c explodingClient) ContainerInspect(ctx context.Context, containerID string, options client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{}, c.err
}
func (c explodingClient) ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error) {
	return client.ContainerListResult{}, c.err
}
func (c explodingClient) ContainerRestartNoWait(ctx context.Context, containerID string) error {
	return c.err
}
func (c explodingClient) Run(ctx context.Context, opts RunConfig) (RunResult, error) {
	return RunResult{}, c.err
}
func (c explodingClient) ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, in io.Reader, out io.Writer) error {
	return c.err
}
func (c explodingClient) ImagePull(_ context.Context, _ reference.Named) (reference.Canonical, error) {
	return nil, c.err
}
func (c explodingClient) ImagePush(ctx context.Context, ref reference.NamedTagged) (io.ReadCloser, error) {
	return nil, c.err
}
func (c explodingClient) ImageBuild(ctx context.Context, g *errgroup.Group, buildContext io.Reader, options BuildOptions) (client.ImageBuildResult, error) {
	return client.ImageBuildResult{}, c.err
}
func (c explodingClient) ImageTag(ctx context.Context, options client.ImageTagOptions) (client.ImageTagResult, error) {
	return client.ImageTagResult{}, c.err
}
func (c explodingClient) ImageInspect(ctx context.Context, imageID string, inspectOpts ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	return client.ImageInspectResult{}, c.err
}
func (c explodingClient) ImageList(ctx context.Context, options client.ImageListOptions) (client.ImageListResult, error) {
	return client.ImageListResult{}, c.err
}
func (c explodingClient) ImageRemove(ctx context.Context, imageID string, options client.ImageRemoveOptions) (client.ImageRemoveResult, error) {
	return client.ImageRemoveResult{}, c.err
}
func (c explodingClient) NewVersionError(ctx context.Context, apiRequired, feature string) error {
	return c.err
}
func (c explodingClient) BuildCachePrune(ctx context.Context, opts client.BuildCachePruneOptions) (client.BuildCachePruneResult, error) {
	return client.BuildCachePruneResult{}, c.err
}
func (c explodingClient) ContainerPrune(ctx context.Context, opts client.ContainerPruneOptions) (client.ContainerPruneResult, error) {
	return client.ContainerPruneResult{}, c.err
}
func (c explodingClient) DaemonInfo(ctx context.Context) (system.Info, error) {
	return system.Info{}, c.err
}

var _ Client = &explodingClient{}
