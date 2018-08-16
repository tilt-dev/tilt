package engine

import (
	"context"
	"github.com/windmilleng/tilt/internal/model"
	"io"
)

func UpService(ctx context.Context, service model.Service, watch bool, stdout io.Writer, stderr io.Writer) error {
	bad, err := NewLocalBuildAndDeployer()
	if err != nil {
		return err
	}
	buildToken, err := bad.BuildAndDeploy(ctx, service, nil)
	if watch {
		for {
			// TODO(matt) actually wait for a file to change instead of building on a loop
			buildToken, err = bad.BuildAndDeploy(ctx, service, buildToken)
			if err != nil {
				return err
			}
		}
	}
	return err
}
