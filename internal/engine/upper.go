package engine

import (
	"context"
	"io"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"
)

func UpService(ctx context.Context, service model.Service, watch bool, stdout io.Writer, stderr io.Writer, manager service.Manager) error {
	bad, err := NewLocalBuildAndDeployer(manager)
	if err != nil {
		return err
	}
	buildToken, err := bad.BuildAndDeploy(ctx, service, nil)
	if watch {
		for {
			// TODO(matt) actually wait for a file to change instead of building on a loop
			logger.Get(ctx).Verbose("building and deploying")
			buildToken, err = bad.BuildAndDeploy(ctx, service, buildToken)
			if err != nil {
				return err
			}
		}
	}
	return err
}
