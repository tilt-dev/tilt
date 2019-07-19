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
	CanUpdateSpecs(specs []model.TargetSpec, env k8s.Env) (canUpd bool, msg string, silent bool)
}

// Differentiate between the CU we'll need for k8s updates (may be docker || synclet || exec)
// and the one we'll need for DC updates (always docker)
type K8sContainerUpdater ContainerUpdater
type DCContainerUpdater ContainerUpdater

func ProvideK8sContainerUpdater(kCli k8s.Client, dCli docker.Client, sm SyncletManager, env k8s.Env, updMode mode.UpdateMode, runtime container.Runtime) K8sContainerUpdater {
	if updMode == mode.UpdateModeImage || updMode == mode.UpdateModeNaive {
		return NewNoopUpdater()
	}

	if updMode == mode.UpdateModeKubectlExec {
		return NewExecUpdater(kCli)
	}

	if updMode == mode.UpdateModeContainer || (runtime == container.RuntimeDocker && env.IsLocalCluster()) {
		return NewDockerContainerUpdater(dCli)
	}

	if updMode == mode.UpdateModeSynclet || runtime == container.RuntimeDocker {
		return NewSyncletUpdater(sm)
	}

	return NewExecUpdater(kCli)
}

func ProvideDCContainerUpdater(dCli docker.Client, updMode mode.UpdateMode) DCContainerUpdater {
	if updMode == mode.UpdateModeImage || updMode == mode.UpdateModeNaive {
		return NewNoopUpdater()
	}

	return NewDockerContainerUpdater(dCli)
}
