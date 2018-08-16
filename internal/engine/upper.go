package engine

import (
	"context"
	"github.com/windmilleng/tilt/internal/model"
	"io"
)

func UpService(ctx context.Context, service model.Service, stdout io.Writer, stderr io.Writer) error {
	bad, err := NewLocalBuildAndDeployer()
	if err != nil {
		return err
	}
	_, err = bad.BuildAndDeploy(ctx, service, nil)
	return err
}
