package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

var _ BuildAndDeployer = &LocalResourceExecutor{}

type LocalResourceExecutor struct {
	ib            build.ImageBuilder
	icb           *imageAndCacheBuilder
	k8sClient     k8s.Client
	env           k8s.Env
	runtime       container.Runtime
	analytics     *analytics.TiltAnalytics
	injectSynclet bool
	clock         build.Clock
	kp            KINDPusher
}

func NewLocalResourceExecutor() *LocalResourceExecutor {
	return &LocalResourceExecutor{}
}

func (lre *LocalResourceExecutor) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	/* ✨ TODO ✨
	targets := lre.ExtractLocalTargets
	if len(targets) != 1 {
		return {}, FallBackErr("requires exactly one LocalTarget")
	}

	t := targets[0]
	runCommand(t.Cmd)
	*/
	return store.BuildResultSet{}, nil
}
