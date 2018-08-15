package build

import (
	"context"

	digest "github.com/opencontainers/go-digest"
)

type ServiceName string

type IncrementalDockerBuilder interface {
	// BuildDockerImageWithChanges applies the changed files and reruns the steps
	BuildDockerImageWithChanges(ctx context.Context, changedFiles []string, name ServiceName) (digest.Digest, error)
}
