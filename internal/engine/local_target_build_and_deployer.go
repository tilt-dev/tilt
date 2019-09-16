package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

var _ BuildAndDeployer = &LocalTargetBuildAndDeployer{}

type LocalTargetBuildAndDeployer struct{}

func NewLocalTargetBuildAndDeployer() *LocalTargetBuildAndDeployer {
	return &LocalTargetBuildAndDeployer{}
}

func (ltbad *LocalTargetBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	/* ✨ TODO ✨
	targets := ltbad.ExtractLocalTargets
	if len(targets) != 1 {
		return {}, FallBackErr("requires exactly one LocalTarget")
	}

	t := targets[0]
	runCommand(t.Cmd)
	*/
	return store.BuildResultSet{}, nil
}
