package docker

import (
	"context"
	"io"
	"sync"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// A Cli implementation that lets us switch back and forth between a local
// Docker instance and one that lives in our K8s cluster.

type switchCli struct {
	localCli   LocalClient
	clusterCli ClusterClient
	orc        model.Orchestrator
	mu         sync.Mutex
}

var _ Client = &switchCli{}
var _ CompositeClient = &switchCli{}

func ProvideSwitchCli(clusterCli ClusterClient, localCli LocalClient) CompositeClient {
	return &switchCli{
		localCli:   localCli,
		clusterCli: clusterCli,
		orc:        model.OrchestratorK8s,
	}
}

var orcKey model.Orchestrator

// WithOrchestrator returns a Context with the current orchestrator set.
func WithOrchestrator(ctx context.Context, orc model.Orchestrator) context.Context {
	return context.WithValue(ctx, orcKey, orc)
}

func (c *switchCli) client(ctx context.Context) Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	orc, ok := ctx.Value(orcKey).(model.Orchestrator)
	if ok {
		return c.ForOrchestrator(orc)
	}
	return c.ForOrchestrator(c.orc)
}

func (c *switchCli) SetOrchestrator(orc model.Orchestrator) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.orc = orc
}
func (c *switchCli) ForOrchestrator(orc model.Orchestrator) Client {
	if orc == model.OrchestratorK8s {
		return c.clusterCli
	}
	return c.localCli
}

func (c *switchCli) CheckConnected() error {
	return c.client(context.Background()).CheckConnected()
}
func (c *switchCli) Env() Env {
	return c.client(context.Background()).Env()
}
func (c *switchCli) BuilderVersion() types.BuilderVersion {
	return c.client(context.Background()).BuilderVersion()
}
func (c *switchCli) ServerVersion() types.Version {
	return c.client(context.Background()).ServerVersion()
}
func (c *switchCli) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return c.client(ctx).ContainerInspect(ctx, containerID)
}
func (c *switchCli) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	return c.client(ctx).ContainerList(ctx, options)
}
func (c *switchCli) ContainerRestartNoWait(ctx context.Context, containerID string) error {
	return c.client(ctx).ContainerRestartNoWait(ctx, containerID)
}
func (c *switchCli) Run(ctx context.Context, opts RunConfig) (RunResult, error) {
	return c.client(ctx).Run(ctx, opts)
}
func (c *switchCli) ExecInContainer(ctx context.Context, cID container.ID, cmd model.Cmd, in io.Reader, out io.Writer) error {
	return c.client(ctx).ExecInContainer(ctx, cID, cmd, in, out)
}
func (c *switchCli) ImagePull(ctx context.Context, ref reference.Named) (reference.Canonical, error) {
	return c.client(ctx).ImagePull(ctx, ref)
}
func (c *switchCli) ImagePush(ctx context.Context, ref reference.NamedTagged) (io.ReadCloser, error) {
	return c.client(ctx).ImagePush(ctx, ref)
}
func (c *switchCli) ImageBuild(ctx context.Context, buildContext io.Reader, options BuildOptions) (types.ImageBuildResponse, error) {
	return c.client(ctx).ImageBuild(ctx, buildContext, options)
}
func (c *switchCli) ImageTag(ctx context.Context, source, target string) error {
	return c.client(ctx).ImageTag(ctx, source, target)
}
func (c *switchCli) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	return c.client(ctx).ImageInspectWithRaw(ctx, imageID)
}
func (c *switchCli) ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error) {
	return c.client(ctx).ImageList(ctx, options)
}
func (c *switchCli) ImageRemove(ctx context.Context, imageID string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error) {
	return c.client(ctx).ImageRemove(ctx, imageID, options)
}
func (c *switchCli) NewVersionError(apiRequired, feature string) error {
	return c.client(context.Background()).NewVersionError(apiRequired, feature)
}
func (c *switchCli) BuildCachePrune(ctx context.Context, opts types.BuildCachePruneOptions) (*types.BuildCachePruneReport, error) {
	return c.client(ctx).BuildCachePrune(ctx, opts)
}
func (c *switchCli) ContainersPrune(ctx context.Context, pruneFilters filters.Args) (types.ContainersPruneReport, error) {
	return c.client(ctx).ContainersPrune(ctx, pruneFilters)
}

// CompositeClient
func (c *switchCli) DefaultLocalClient() Client {
	return c.localCli
}
func (c *switchCli) DefaultClusterClient() Client {
	return c.clusterCli
}
func (c *switchCli) ClientFor(cluster v1alpha1.Cluster) Client {
	conn := cluster.Spec.Connection
	if conn.Kubernetes != nil {
		// TODO: pick correct client in multiple cluster situation
		return c.DefaultClusterClient()
	}
	return c.DefaultLocalClient()
}
func (c *switchCli) HasMultipleClients() bool {
	return c.localCli.Env().DaemonHost() != c.clusterCli.Env().DaemonHost()
}
