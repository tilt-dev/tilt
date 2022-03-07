package containerupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ExecUpdater struct {
	kCli k8s.Client
}

var _ ContainerUpdater = &ExecUpdater{}

func NewExecUpdater(kCli k8s.Client) *ExecUpdater {
	return &ExecUpdater{kCli: kCli}
}

func (cu *ExecUpdater) UpdateContainer(ctx context.Context, cInfo liveupdates.Container,
	archiveToCopy io.Reader, filesToDelete []string, cmds []model.Cmd, hotReload bool) error {
	if !hotReload {
		return fmt.Errorf("ExecUpdater does not support `restart_container()` step. If you ran Tilt " +
			"with `--updateMode=exec`, omit this flag. If you are using a non-Docker container runtime, " +
			"see https://github.com/tilt-dev/tilt-extensions/tree/master/restart_process for a workaround")
	}

	l := logger.Get(ctx)
	w := logger.Get(ctx).Writer(logger.InfoLvl)

	// delete files (if any)
	if len(filesToDelete) > 0 {
		buf := bytes.NewBuffer(nil)
		rmWriter := io.MultiWriter(w, buf)
		err := cu.kCli.Exec(ctx,
			cInfo.PodID, cInfo.ContainerName, cInfo.Namespace,
			append([]string{"rm", "-rf"}, filesToDelete...), nil, rmWriter, rmWriter)
		if err != nil {
			return fmt.Errorf("removing old files: %v", handleK8sExecError(buf, err))
		}
	}

	// copy files to container
	buf := bytes.NewBuffer(nil)
	tarWriter := io.MultiWriter(w, buf)
	tarCmd := tarCmd()
	err := cu.kCli.Exec(ctx, cInfo.PodID, cInfo.ContainerName, cInfo.Namespace,
		tarCmd.Argv, archiveToCopy, tarWriter, tarWriter)
	if err != nil {
		if exitCodeErr, ok := build.WrapCodeExitError(err, tarCmd); ok {
			switch exitCodeErr.ExitCode {
			case TarExitCodePermissionDenied:
				return permissionDeniedErr(err)
			case GenericExitCodeCannotExec:
				return cannotExecErr(err)
			}
		}
		return fmt.Errorf("copying changed files: %v", handleK8sExecError(buf, err))
	}

	// run commands
	for i, c := range cmds {
		l.Infof("[CMD %d/%d] %s", i+1, len(cmds), strings.Join(c.Argv, " "))
		err := cu.kCli.Exec(ctx, cInfo.PodID, cInfo.ContainerName, cInfo.Namespace,
			c.Argv, nil, w, w)
		if err != nil {
			if exitCodeErr, ok := build.WrapCodeExitError(err, c); ok {
				return exitCodeErr
			}
			return errors.Wrapf(err, "executing %v on container %s", c, cInfo.ContainerID.ShortStr())
		}

	}

	return nil
}

func handleK8sExecError(out *bytes.Buffer, err error) error {
	msg := strings.ToLower(fmt.Sprintf("%s\n%s", out.String(), err.Error()))
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "cannot open") {
		return permissionDeniedErr(err)
	}
	if strings.Contains(msg, "executable file not found") {
		return cannotExecErr(err)
	}
	return err
}
