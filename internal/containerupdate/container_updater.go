package containerupdate

import (
	"context"
	"fmt"
	"io"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/engine/errors"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/mode"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/target"
)

type ContainerUpdater interface {
	UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
		archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error
	ValidateSpecs(specs []model.TargetSpec, env k8s.Env) error
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

// validation func shared by ExecUpdater and SyncletUpdater
func validateSpecsOnlyImagesDeployedToK8s(specs []model.TargetSpec, updaterName string) error {
	// If you've got DC specs at this point, something is very wrong -- we should never
	// have routed them through the BuildOrder containing this container updater.
	isDC := len(model.ExtractDockerComposeTargets(specs)) > 0
	if isDC {
		return fmt.Errorf("%s does not support DockerCompose targets (this should never happen: please contact Tilt support)", updaterName)
	}

	if !target.AllImagesDeployerToK8s(specs) {
		return errors.RedirectToNextBuilderInfof("%s can only handle images deployed to k8s (i.e. not base images)", updaterName)
	}

	return nil
}
