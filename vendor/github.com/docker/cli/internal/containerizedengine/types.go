package containerizedengine

import (
	"context"
	"errors"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
	registryclient "github.com/docker/cli/cli/registry/client"
	"github.com/docker/docker/api/types"
	ver "github.com/hashicorp/go-version"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	// CommunityEngineImage is the repo name for the community engine
	CommunityEngineImage = "engine-community"

	// EnterpriseEngineImage is the repo name for the enterprise engine
	EnterpriseEngineImage = "engine-enterprise"

	containerdSockPath  = "/run/containerd/containerd.sock"
	engineContainerName = "dockerd"
	engineNamespace     = "docker"

	// Used to signal the containerd-proxy if it should manage
	proxyLabel = "com.docker/containerd-proxy.scope"
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

	// ErrEngineImageMissingTag returned if the engine image is missing the version tag
	ErrEngineImageMissingTag = errors.New("malformed engine image missing tag")

	engineSpec = specs.Spec{
		Root: &specs.Root{
			Path: "rootfs",
		},
		Process: &specs.Process{
			Cwd: "/",
			Args: []string{
				// In general, configuration should be driven by the config file, not these flags
				// TODO - consider moving more of these to the config file, and make sure the defaults are set if not present.
				"/sbin/dockerd",
				"-s",
				"overlay2",
				"--containerd",
				"/run/containerd/containerd.sock",
				"--default-runtime",
				"containerd",
				"--add-runtime",
				"containerd=runc",
			},
			User: specs.User{
				UID: 0,
				GID: 0,
			},
			Env: []string{
				"PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin",
			},
			NoNewPrivileges: false,
		},
	}
)

// Client can be used to manage the lifecycle of
// dockerd running as a container on containerd.
type Client interface {
	Close() error
	ActivateEngine(ctx context.Context,
		opts EngineInitOptions,
		out OutStream,
		authConfig *types.AuthConfig,
		healthfn func(context.Context) error) error
	InitEngine(ctx context.Context,
		opts EngineInitOptions,
		out OutStream,
		authConfig *types.AuthConfig,
		healthfn func(context.Context) error) error
	DoUpdate(ctx context.Context,
		opts EngineInitOptions,
		out OutStream,
		authConfig *types.AuthConfig,
		healthfn func(context.Context) error) error
	GetEngineVersions(ctx context.Context, registryClient registryclient.RegistryClient, currentVersion, imageName string) (AvailableVersions, error)

	GetEngine(ctx context.Context) (containerd.Container, error)
	RemoveEngine(ctx context.Context, engine containerd.Container) error
	GetCurrentEngineVersion(ctx context.Context) (EngineInitOptions, error)
}
type baseClient struct {
	cclient containerdClient
}

// EngineInitOptions contains the configuration settings
// use during initialization of a containerized docker engine
type EngineInitOptions struct {
	RegistryPrefix string
	EngineImage    string
	EngineVersion  string
	ConfigFile     string
	scope          string
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
}

// AvailableVersions groups the available versions which were discovered
type AvailableVersions struct {
	Downgrades []DockerVersion
	Patches    []DockerVersion
	Upgrades   []DockerVersion
}

// DockerVersion wraps a semantic version to retain the original tag
// since the docker date based versions don't strictly follow semantic
// versioning (leading zeros, etc.)
type DockerVersion struct {
	ver.Version
	Tag string
}

// Update stores available updates for rendering in a table
type Update struct {
	Type    string
	Version string
	Notes   string
}

// OutStream is an output stream used to write normal program output.
type OutStream interface {
	io.Writer
	FD() uintptr
	IsTerminal() bool
}
