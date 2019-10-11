package types

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	ver "github.com/hashicorp/go-version"
)

const (
	// CommunityEngineImage is the repo name for the community engine
	CommunityEngineImage = "engine-community"

	// EnterpriseEngineImage is the repo name for the enterprise engine
	EnterpriseEngineImage = "engine-enterprise"

	// RegistryPrefix is the default prefix used to pull engine images
	RegistryPrefix = "docker.io/store/docker"

	// ReleaseNotePrefix is where to point users to for release notes
	ReleaseNotePrefix = "https://docs.docker.com/releasenotes"

	// RuntimeMetadataName is the name of the runtime metadata file
	// When stored as a label on the container it is prefixed by "com.docker."
	RuntimeMetadataName = "distribution_based_engine"
)

// ContainerizedClient can be used to manage the lifecycle of
// dockerd running as a container on containerd.
type ContainerizedClient interface {
	Close() error
	ActivateEngine(ctx context.Context,
		opts EngineInitOptions,
		out OutStream,
		authConfig *types.AuthConfig) error
	DoUpdate(ctx context.Context,
		opts EngineInitOptions,
		out OutStream,
		authConfig *types.AuthConfig) error
}

// EngineInitOptions contains the configuration settings
// use during initialization of a containerized docker engine
type EngineInitOptions struct {
	RegistryPrefix     string
	EngineImage        string
	EngineVersion      string
	ConfigFile         string
	RuntimeMetadataDir string
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

// RuntimeMetadata holds platform information about the daemon
type RuntimeMetadata struct {
	Platform             string `json:"platform"`
	ContainerdMinVersion string `json:"containerd_min_version"`
	Runtime              string `json:"runtime"`
	EngineImage          string `json:"engine_image"`
}
