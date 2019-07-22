package containerupdate

import (
	"context"
	"fmt"
	"io"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/mode"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/target"
)

type ContainerUpdater interface {
	UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
		archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error
	SupportsSpecs(specs []model.TargetSpec) (supported bool, msg string)
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
		return NewDockerContainerUpdater(dCli, env, runtime)
	}

	if updMode == mode.UpdateModeSynclet || runtime == container.RuntimeDocker {
		return NewSyncletUpdater(sm)
	}

	return NewExecUpdater(kCli)
}

func ProvideDCContainerUpdater(dCli docker.Client, updMode mode.UpdateMode, env k8s.Env, runtime container.Runtime) DCContainerUpdater {
	if updMode == mode.UpdateModeImage || updMode == mode.UpdateModeNaive {
		return NewNoopUpdater()
	}

	return NewDockerContainerUpdater(dCli, env, runtime)
}

// validation func shared by ExecUpdater and SyncletUpdater
func specsAreOnlyImagesDeployedToK8s(specs []model.TargetSpec, updaterName string) (ok bool, msg string) {
	// If you've got DC specs at this point, something is very wrong -- we should never
	// have routed them through the BuildOrder containing this container updater.
	isDC := len(model.ExtractDockerComposeTargets(specs)) > 0
	if isDC {
		return false, fmt.Sprintf("%s does not support DockerCompose targets (this should never happen: please contact Tilt support)", updaterName)
	}

	if !target.AllImagesDeployedToK8s(specs) {
		// We ought filter out non-deployed images when we create the LiveUpdateStateSet,
		// so this should never happen -- implies something has gone wrong internally.
		return false, fmt.Sprintf("%s can only handle images deployed to k8s (i.e. not base images)", updaterName)
	}

	return true, ""
}
