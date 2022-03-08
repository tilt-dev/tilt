package containerupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

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
		cmd := model.Cmd{Argv: append([]string{"rm", "-rf"}, filesToDelete...)}
		err := cu.kCli.Exec(ctx,
			cInfo.PodID, cInfo.ContainerName, cInfo.Namespace,
			cmd.Argv, nil, rmWriter, rmWriter)
		if err != nil {
			return wrapK8sTarErr(buf, err, cmd, "removing old files")
		}
	}

	// copy files to container
	buf := bytes.NewBuffer(nil)
	tarWriter := io.MultiWriter(w, buf)
	tarCmd := tarCmd()
	err := cu.kCli.Exec(ctx, cInfo.PodID, cInfo.ContainerName, cInfo.Namespace,
		tarCmd.Argv, archiveToCopy, tarWriter, tarWriter)
	if err != nil {
		return wrapK8sTarErr(buf, err, tarCmd, "copying changed files")
	}

	// run commands
	for i, c := range cmds {
		l.Infof("[CMD %d/%d] %s", i+1, len(cmds), strings.Join(c.Argv, " "))
		err := cu.kCli.Exec(ctx, cInfo.PodID, cInfo.ContainerName, cInfo.Namespace,
			c.Argv, nil, w, w)
		if err != nil {
			return fmt.Errorf(
				"executing on container %s: %w",
				cInfo.ContainerID.ShortStr(),
				wrapRunStepError(wrapK8sGenericExecErr(err, c)),
			)
		}

	}

	return nil
}

// wrapK8sTarErr provides user-friendly diagnostics for common failures when
// running `tar` as part of a Live Update.
func wrapK8sTarErr(out *bytes.Buffer, err error, cmd model.Cmd, action string) error {
	if exitCode, ok := ExtractExitCode(err); ok {
		return wrapTarExecErr(err, cmd, exitCode)
	}

	// if we didn't get an explicit exit code from the k8s error, look at the
	// error text + stdout/stderr to see if it's a failure case we understand
	msg := strings.ToLower(fmt.Sprintf("%s\n%s", out.String(), err.Error()))
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "cannot open") {
		return permissionDeniedErr(err)
	}
	if strings.Contains(msg, "executable file not found") {
		return cannotExecErr(err)
	}
	return fmt.Errorf("%s: %w", action, err)
}

// wrapK8sGenericExecErr massages exec errors to be more user-friendly.
func wrapK8sGenericExecErr(err error, cmd model.Cmd) error {
	if exitCode, ok := ExtractExitCode(err); ok {
		return NewExecError(cmd, exitCode)
	}

	if strings.Contains(err.Error(), "executable file not found") {
		return NewExecError(cmd, GenericExitCodeNotFound)
	}
	return err
}
