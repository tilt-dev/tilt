package engine

import (
	"context"
	"fmt"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

var _ BuildAndDeployer = &SyncletBuildAndDeployer{}

type SyncletBuildAndDeployer struct{}

func NewSyncletBuildAndDeployer() *SyncletBuildAndDeployer {
	return &SyncletBuildAndDeployer{}
}

func (sbd *SyncletBuildAndDeployer) BuildAndDeploy(ctx context.Context, service model.Service, state BuildState) (BuildResult, error) {
	return BuildResult{}, fmt.Errorf("you haven't implemented me yet :(")
}

func (sbd *SyncletBuildAndDeployer) GetContainerForBuild(ctx context.Context, build BuildResult) (k8s.ContainerID, error) {
	return "", nil
}
