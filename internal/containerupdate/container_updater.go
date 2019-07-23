package containerupdate

import (
	"context"
	"io"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/mode"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type ContainerUpdater interface {
	UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
		archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error
	SupportsSpecs(specs []model.TargetSpec) (supported bool, msg string)
}

func ProvideContainerUpdater(kCli k8s.Client, dCli docker.Client, sm SyncletManager, env k8s.Env, updMode mode.UpdateMode, runtime container.Runtime) ContainerUpdater {
	if updMode == mode.UpdateModeImage || updMode == mode.UpdateModeNaive {
		return NewExplodingContainerUpdater()
	}

	if updMode == mode.UpdateModeKubectlExec {
		return NewExecUpdater(kCli)
	}

	if updMode == mode.UpdateModeContainer || (runtime == container.RuntimeDocker && env.IsLocalCluster()) {
		return NewDockerContainerUpdater(dCli, env, runtime)
	}

	if updMode == mode.UpdateModeSynclet || runtime == container.RuntimeDocker {
		return NewSyncletUpdater(sm)
	}

	return NewExecUpdater(kCli)
}
