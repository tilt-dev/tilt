package containerupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/k8s"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type DockerContainerUpdater struct {
	dCli docker.Client
}

var _ ContainerUpdater = &DockerContainerUpdater{}

func NewDockerContainerUpdater(dCli docker.Client) ContainerUpdater {
	return &DockerContainerUpdater{dCli: dCli}
}

func (cu *DockerContainerUpdater) CanUpdateSpecs(specs []model.TargetSpec, env k8s.Env) (canUpd bool, msg string, silent bool) {
	isDC := len(model.ExtractDockerComposeTargets(specs)) > 0
	isK8s := len(model.ExtractK8sTargets(specs)) > 0
	canLocalUpdate := isDC || (isK8s && env.IsLocalCluster())
	if !canLocalUpdate {
		return false, "Local container builder needs docker-compose or k8s cluster w/ local updates", true
	}
	return true, "", false
}

func (cu *DockerContainerUpdater) UpdateContainer(ctx context.Context, deployInfo store.DeployInfo,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	l := logger.Get(ctx)

	err := cu.rmPathsFromContainer(ctx, deployInfo.ContainerID, filesToDelete)
	if err != nil {
		return errors.Wrap(err, "rmPathsFromContainer")
	}

	// TODO(maia): catch errors -- CopyToContainer doesn't return errors if e.g. it
	// fails to write a file b/c of permissions =(
	err = cu.dCli.CopyToContainerRoot(ctx, deployInfo.ContainerID.String(), archiveToCopy)
	if err != nil {
		return err
	}

	// Exec run's on container
	for _, s := range cmds {
		err = cu.dCli.ExecInContainer(ctx, deployInfo.ContainerID, s, l.Writer(logger.InfoLvl))
		if err != nil {
			return build.WrapContainerExecError(err, deployInfo.ContainerID, s)
		}
	}

	if hotReload {
		l.Debugf("Hot reload on, skipping container restart: %s", deployInfo.ContainerID.ShortStr())
		return nil
	}

	// Restart container so that entrypoint restarts with the updated files etc.
	l.Debugf("Restarting container: %s", deployInfo.ContainerID.ShortStr())
	err = cu.dCli.ContainerRestartNoWait(ctx, deployInfo.ContainerID.String())
	if err != nil {
		return errors.Wrap(err, "ContainerRestart")
	}
	return nil
}

func (cu *DockerContainerUpdater) rmPathsFromContainer(ctx context.Context, cID container.ID, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	out := bytes.NewBuffer(nil)
	err := cu.dCli.ExecInContainer(ctx, cID, model.Cmd{Argv: makeRmCmd(paths)}, out)
	if err != nil {
		if docker.IsExitError(err) {
			return fmt.Errorf("Error deleting files from container: %s", out.String())
		}
		return errors.Wrap(err, "Error deleting files from container")
	}
	return nil
}

func makeRmCmd(paths []string) []string {
	cmd := []string{"rm", "-rf"}
	for _, p := range paths {
		cmd = append(cmd, p)
	}
	return cmd
}
