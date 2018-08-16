package build

import (
	"context"

	"github.com/windmilleng/tilt/internal/model"

	digest "github.com/opencontainers/go-digest"
)

type IncrementalDockerBuilder interface {
	// BuildDockerImageWithChanges applies the changed files and reruns the steps
	BuildDockerImageWithChanges(ctx context.Context, changedFiles []string, name model.ServiceName) (digest.Digest, error)
}
