package engine

import (
	"context"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

type buildToken struct {
	n        reference.NamedTagged
	entities []k8s.K8sEntity
}

func (b *buildToken) isEmpty() bool {
	return b == nil
}

type BuildAndDeployer interface {
	// Builds and deployed the specified service.
	// Returns a buildToken that can be passed on successive calls to allow incremental builds.
	// If buildToken is passed and changedFiles is non-nil, changedFiles should specify the list of files that have
	//   changed since the last build.
	BuildAndDeploy(ctx context.Context, service model.Service, token *buildToken, changedFiles []string) (*buildToken, error)
}
