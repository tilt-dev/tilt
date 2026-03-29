package docker

import (
	"context"
	"io"
	"sync"

	"github.com/distribution/reference"
	typesbuild "github.com/moby/moby/api/types/build"
	"github.com/moby/moby/client"
	"golang.org/x/sync/errgroup"

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
func (c *switchCli) BuilderVersion(ctx context.Context) (typesbuild.BuilderVersion, error) {
	return c.client(ctx).BuilderVersion(ctx)
}
func (c *switchCli) ServerVersion(ctx context.Context) (client.ServerVersionResult, error) {
	return c.client(ctx).ServerVersion(ctx)
}
func (c *switchCli) ContainerLogs(ctx context.Context, containerID string, options client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	return c.client(ctx).ContainerLogs(ctx, containerID, options)
}
func (c *switchCli) ContainerInspect(ctx context.Context, containerID string, options client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return c.client(ctx).ContainerInspect(ctx, containerID, options)
}
func (c *switchCli) ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error) {
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
func (c *switchCli) ImageBuild(ctx context.Context, g *errgroup.Group, buildContext io.Reader, options BuildOptions) (client.ImageBuildResult, error) {
	return c.client(ctx).ImageBuild(ctx, g, buildContext, options)
}
func (c *switchCli) ImageTag(ctx context.Context, options client.ImageTagOptions) (client.ImageTagResult, error) {
	return c.client(ctx).ImageTag(ctx, options)
}
func (c *switchCli) ImageInspect(ctx context.Context, imageID string, inspectOpts ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	return c.client(ctx).ImageInspect(ctx, imageID, inspectOpts...)
}
func (c *switchCli) ImageList(ctx context.Context, options client.ImageListOptions) (client.ImageListResult, error) {
	return c.client(ctx).ImageList(ctx, options)
}
func (c *switchCli) ImageRemove(ctx context.Context, imageID string, options client.ImageRemoveOptions) (client.ImageRemoveResult, error) {
	return c.client(ctx).ImageRemove(ctx, imageID, options)
}
func (c *switchCli) NewVersionError(ctx context.Context, apiRequired, feature string) error {
	return c.client(context.Background()).NewVersionError(ctx, apiRequired, feature)
}
func (c *switchCli) BuildCachePrune(ctx context.Context, opts client.BuildCachePruneOptions) (client.BuildCachePruneResult, error) {
	return c.client(ctx).BuildCachePrune(ctx, opts)
}
func (c *switchCli) ContainerPrune(ctx context.Context, opts client.ContainerPruneOptions) (client.ContainerPruneResult, error) {
	return c.client(ctx).ContainerPrune(ctx, opts)
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
