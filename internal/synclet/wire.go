// +build wireinject
// The build tag makes sure the stub is not built in the final build.

package synclet

import (
	"context"

	"github.com/google/wire"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
)

func WireSynclet(ctx context.Context, runtime container.Runtime) (*Synclet, error) {
	wire.Build(
		docker.LocalWireSet,
		NewSynclet,
	)
	return nil, nil
}
