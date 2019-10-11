package containerizedengine

import (
	"context"
	"errors"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
)

const (
	containerdSockPath = "/run/containerd/containerd.sock"
	engineNamespace    = "com.docker"
)

var (
	// ErrEngineAlreadyPresent returned when engine already present and should not be
	ErrEngineAlreadyPresent = errors.New("engine already present, use the update command to change versions")

	// ErrEngineNotPresent returned when the engine is not present and should be
	ErrEngineNotPresent = errors.New("engine not present")

	// ErrMalformedConfigFileParam returned if the engine config file parameter is malformed
	ErrMalformedConfigFileParam = errors.New("malformed --config-file param on engine")

	// ErrEngineConfigLookupFailure returned if unable to lookup existing engine configuration
	ErrEngineConfigLookupFailure = errors.New("unable to lookup existing engine configuration")

	// ErrEngineShutdownTimeout returned if the engine failed to shutdown in time
	ErrEngineShutdownTimeout = errors.New("timeout waiting for engine to exit")
)

type baseClient struct {
	cclient containerdClient
}

// containerdClient abstracts the containerd client to aid in testability
type containerdClient interface {
	Containers(ctx context.Context, filters ...string) ([]containerd.Container, error)
	NewContainer(ctx context.Context, id string, opts ...containerd.NewContainerOpts) (containerd.Container, error)
	Pull(ctx context.Context, ref string, opts ...containerd.RemoteOpt) (containerd.Image, error)
	GetImage(ctx context.Context, ref string) (containerd.Image, error)
	Close() error
	ContentStore() content.Store
	ContainerService() containers.Store
	Install(context.Context, containerd.Image, ...containerd.InstallOpts) error
	Version(ctx context.Context) (containerd.Version, error)
}
